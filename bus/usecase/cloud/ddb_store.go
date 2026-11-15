package cloud

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	attrName  = "image_object_name"
	attrSort  = "werewolf_sort_key"
	attrUUID  = "image_object_uuid"
	indexUUID = "image_object_uuid"

	// a transaction writes at most 100 items
	maxTransactItems = 100
	ddbTimeout       = time.Minute
)

// errConflict: a conditional put found the item.
var errConflict = errors.New("dynamodb item already exists")

// scanFilter picks entity rows; the fields are ANDed, an empty scanFilter picks every entity row.
type scanFilter struct {
	ImageUUID        string   // image_uuid: the image itself and its adversarial objects
	AdversarialUUIDs []string // adversarial_uuid: the filters of these adversarial objects
}

// store is the access to the table; DbRealizationCloud knows nothing about expressions.
type store interface {
	// byUUID: every row of the object with the uuid, the entity row and its prediction rows.
	byUUID(ctx context.Context, id string) ([]item, error)
	// byName: the rows of the partition; entitiesOnly leaves out the prediction rows.
	byName(ctx context.Context, name string, entitiesOnly bool) ([]item, error)
	scanEntities(ctx context.Context, f scanFilter) ([]item, error)
	write(ctx context.Context, w *writeSet) error
}

// ddbAPI is the part of the dynamodb client used by the store.
type ddbAPI interface {
	Query(ctx context.Context, in *dynamodb.QueryInput, opts ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
	Scan(ctx context.Context, in *dynamodb.ScanInput, opts ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
	TransactWriteItems(ctx context.Context, in *dynamodb.TransactWriteItemsInput, opts ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error)
}

var _ ddbAPI = (*dynamodb.Client)(nil)

type ddbStore struct {
	api   ddbAPI
	table string
}

// byUUID reads the global secondary index, which is eventually consistent.
func (s *ddbStore) byUUID(ctx context.Context, id string) ([]item, error) {
	expr, err := expression.NewBuilder().
		WithKeyCondition(expression.Key(attrUUID).Equal(expression.Value(id))).Build()
	if err != nil {
		return nil, err
	}
	return s.query(ctx, &dynamodb.QueryInput{
		TableName:                 aws.String(s.table),
		IndexName:                 aws.String(indexUUID),
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	})
}

func (s *ddbStore) byName(ctx context.Context, name string, entitiesOnly bool) ([]item, error) {
	cond := expression.Key(attrName).Equal(expression.Value(name))
	if entitiesOnly {
		cond = cond.And(expression.Key(attrSort).BeginsWith(entitySortPrefix))
	}
	expr, err := expression.NewBuilder().WithKeyCondition(cond).Build()
	if err != nil {
		return nil, err
	}
	return s.query(ctx, &dynamodb.QueryInput{
		TableName:                 aws.String(s.table),
		ConsistentRead:            aws.Bool(true),
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	})
}

func (s *ddbStore) query(ctx context.Context, in *dynamodb.QueryInput) ([]item, error) {
	ctx, cancel := context.WithTimeout(ctx, ddbTimeout)
	defer cancel()

	var out []item
	for {
		res, err := s.api.Query(ctx, in)
		if err != nil {
			return nil, fmt.Errorf("query dynamodb table %s: %w", s.table, err)
		}
		var page []item
		if err = attributevalue.UnmarshalListOfMaps(res.Items, &page); err != nil {
			return nil, err
		}
		out = append(out, page...)
		if len(res.LastEvaluatedKey) == 0 {
			return out, nil
		}
		in.ExclusiveStartKey = res.LastEvaluatedKey
	}
}

func (s *ddbStore) scanEntities(ctx context.Context, f scanFilter) ([]item, error) {
	cond := expression.Name(attrSort).BeginsWith(entitySortPrefix)
	if f.ImageUUID != "" {
		cond = expression.And(cond, expression.Name("image_uuid").Equal(expression.Value(f.ImageUUID)))
	}
	if len(f.AdversarialUUIDs) > 0 {
		ops := make([]expression.OperandBuilder, len(f.AdversarialUUIDs))
		for i, id := range f.AdversarialUUIDs {
			ops[i] = expression.Value(id)
		}
		cond = expression.And(cond, expression.Name("adversarial_uuid").In(ops[0], ops[1:]...))
	}
	expr, err := expression.NewBuilder().WithFilter(cond).Build()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, ddbTimeout)
	defer cancel()

	in := &dynamodb.ScanInput{
		TableName:                 aws.String(s.table),
		ConsistentRead:            aws.Bool(true),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}
	var out []item
	for {
		res, err := s.api.Scan(ctx, in)
		if err != nil {
			return nil, fmt.Errorf("scan dynamodb table %s: %w", s.table, err)
		}
		var page []item
		if err = attributevalue.UnmarshalListOfMaps(res.Items, &page); err != nil {
			return nil, err
		}
		out = append(out, page...)
		if len(res.LastEvaluatedKey) == 0 {
			return out, nil
		}
		in.ExclusiveStartKey = res.LastEvaluatedKey
	}
}

// write is atomic for up to maxTransactItems items; a bigger set goes in several transactions.
func (s *ddbStore) write(ctx context.Context, w *writeSet) error {
	ops := make([]types.TransactWriteItem, 0, len(w.puts)+len(w.dels))
	for _, p := range w.puts {
		op, err := s.putOp(p)
		if err != nil {
			return err
		}
		ops = append(ops, op)
	}
	for k := range w.dels {
		ops = append(ops, types.TransactWriteItem{Delete: &types.Delete{
			TableName: aws.String(s.table),
			Key: map[string]types.AttributeValue{
				attrName: &types.AttributeValueMemberS{Value: k.name},
				attrSort: &types.AttributeValueMemberS{Value: k.sortKey},
			},
		}})
	}

	ctx, cancel := context.WithTimeout(ctx, ddbTimeout)
	defer cancel()

	for len(ops) > 0 {
		n := min(len(ops), maxTransactItems)
		if _, err := s.api.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: ops[:n]}); err != nil {
			return mapWriteErr(err)
		}
		ops = ops[n:]
	}
	return nil
}

func (s *ddbStore) putOp(p putItem) (types.TransactWriteItem, error) {
	av, err := attributevalue.MarshalMap(p.it)
	if err != nil {
		return types.TransactWriteItem{}, err
	}
	put := &types.Put{TableName: aws.String(s.table), Item: av}
	if p.mustBeNew {
		expr, err := expression.NewBuilder().
			WithCondition(expression.AttributeNotExists(expression.Name(attrName))).Build()
		if err != nil {
			return types.TransactWriteItem{}, err
		}
		put.ConditionExpression = expr.Condition()
		put.ExpressionAttributeNames = expr.Names()
		put.ExpressionAttributeValues = expr.Values()
	}
	return types.TransactWriteItem{Put: put}, nil
}

func mapWriteErr(err error) error {
	var canceled *types.TransactionCanceledException
	if errors.As(err, &canceled) {
		for _, r := range canceled.CancellationReasons {
			if aws.ToString(r.Code) == "ConditionalCheckFailed" {
				return errConflict
			}
		}
	}
	return fmt.Errorf("write dynamodb items: %w", err)
}
