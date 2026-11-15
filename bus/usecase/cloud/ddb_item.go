package cloud

import (
	"fmt"
	"sort"
	"time"

	"github.com/belobragin/werewolf/bus/pkg/model/alter_model"
	"github.com/belobragin/werewolf/bus/pkg/model/diagnose_model"
	"github.com/belobragin/werewolf/bus/pkg/model/picture_model"
	stt "github.com/belobragin/werewolf/bus/pkg/storage"
)

// Rows of the one table. The partition key is the name of an image, an adversarial object or a filter.
// An entity row (the object itself) has no rank, so its sort key starts with "_"; a prediction row
// (the diagnose_result of one rank) has the rank in front. The attribute names are the column names
// of the wfn tables, except wfn.diagnose_result.result, which is diagnose_result here.
const entitySortPrefix = "_"

// tsLayout is fixed-width UTC, so the timestamps sort as strings.
const tsLayout = "2006-01-02T15:04:05.000000000Z"

func nowString() string { return time.Now().UTC().Format(tsLayout) }

// sortKey is "<rank>_<algorythm>_<alter_goal>"; an empty goal leaves the trailing "_".
func sortKey(rank int, algorythm, alterGoal string) string {
	return fmt.Sprintf("%d_%s_%s", rank, algorythm, alterGoal)
}

// entitySortKey is the same string without a rank, as an object has no rank.
func entitySortKey(algorythm, alterGoal string) string {
	return fmt.Sprintf("_%s_%s", algorythm, alterGoal)
}

type key struct{ name, sortKey string }

type item struct {
	ImageObjectName string `dynamodbav:"image_object_name"`
	WerewolfSortKey string `dynamodbav:"werewolf_sort_key"`
	ImageObjectUUID string `dynamodbav:"image_object_uuid"`

	// the uuid of the image itself, for an adversarial object the uuid of its source image
	ImageUUID        string `dynamodbav:"image_uuid,omitempty"`
	ImageName        string `dynamodbav:"image_name,omitempty"`
	ImageDescription string `dynamodbav:"image_description,omitempty"`
	MimeType         string `dynamodbav:"mime_type,omitempty"`
	ImageSize        string `dynamodbav:"image_size,omitempty"`
	ImageHeight      int    `dynamodbav:"image_height,omitempty"`
	ImageWidth       int    `dynamodbav:"image_width,omitempty"`

	AdversarialObjectUUID string `dynamodbav:"adversarial_object_uuid,omitempty"`
	AdversarialObjectName string `dynamodbav:"adversarial_object_name,omitempty"`
	WithFilterObject      bool   `dynamodbav:"with_filter_object,omitempty"`
	// a filter keeps the algorythm and the alter_goal of its adversarial object
	Algorythm string `dynamodbav:"algorythm,omitempty"`
	AlterGoal string `dynamodbav:"alter_goal,omitempty"`

	FilterObjectUUID string `dynamodbav:"filter_object_uuid,omitempty"`
	FilterObjectName string `dynamodbav:"filter_object_name,omitempty"`
	// the source adversarial object of a filter
	AdversarialUUID string `dynamodbav:"adversarial_uuid,omitempty"`

	PredictionRank *int                                           `dynamodbav:"prediction_rank,omitempty"`
	DiagnoseResult map[string]diagnose_model.ResnetPredictionData `dynamodbav:"diagnose_result,omitempty"`

	CreatedAt string `dynamodbav:"created_at,omitempty"`
	UpdatedAt string `dynamodbav:"updated_at,omitempty"`
}

func (it *item) key() key { return key{it.ImageObjectName, it.WerewolfSortKey} }

// kind is the kind of an entity row; a prediction row has none.
func (it *item) kind() stt.ObjectKind {
	if it.PredictionRank != nil {
		return ""
	}
	switch {
	case it.ImageName != "":
		return stt.KindImage
	case it.AdversarialObjectName != "":
		return stt.KindAdversarial
	case it.FilterObjectName != "":
		return stt.KindFilter
	}
	return ""
}

func newImageItem(r *picture_model.UploadReply, now string) item {
	id := r.ImageID.String()
	return item{
		ImageObjectName:  r.ImageName,
		WerewolfSortKey:  entitySortKey("", ""),
		ImageObjectUUID:  id,
		ImageUUID:        id,
		ImageName:        r.ImageName,
		ImageDescription: r.ImageDescription,
		MimeType:         r.MimeType,
		ImageSize:        r.ImageSize,
		ImageHeight:      r.ImageHeight,
		ImageWidth:       r.ImageWidth,
		CreatedAt:        now,
	}
}

func newAdversarialItem(m *alter_model.AlterPostToDb, now string) item {
	id := m.AdversarialObjectID.String()
	return item{
		ImageObjectName:       m.AdversarialName,
		WerewolfSortKey:       entitySortKey(m.Algorythm, m.AlterGoal),
		ImageObjectUUID:       id,
		ImageUUID:             m.ImageID.String(),
		AdversarialObjectUUID: id,
		AdversarialObjectName: m.AdversarialName,
		WithFilterObject:      m.HasFilter(),
		Algorythm:             m.Algorythm,
		AlterGoal:             m.AlterGoal,
		CreatedAt:             now,
	}
}

func newFilterItem(m *alter_model.AlterPostToDb, now string) item {
	id := m.FilterObjectID.String()
	return item{
		ImageObjectName:  m.FilterName,
		WerewolfSortKey:  entitySortKey(m.Algorythm, m.AlterGoal),
		ImageObjectUUID:  id,
		FilterObjectUUID: id,
		FilterObjectName: m.FilterName,
		AdversarialUUID:  m.AdversarialObjectID.String(),
		Algorythm:        m.Algorythm,
		AlterGoal:        m.AlterGoal,
		CreatedAt:        now,
	}
}

// newPredictionItem is the row of one rank of the owner, an image or an adversarial object.
func newPredictionItem(owner item, rank int, p map[string]diagnose_model.ResnetPredictionData, created, updated string) item {
	it := item{
		ImageObjectName: owner.ImageObjectName,
		WerewolfSortKey: sortKey(rank, owner.Algorythm, owner.AlterGoal),
		ImageObjectUUID: owner.ImageObjectUUID,
		PredictionRank:  &rank,
		DiagnoseResult:  p,
		CreatedAt:       created,
		UpdatedAt:       updated,
	}
	if owner.kind() == stt.KindAdversarial {
		it.AdversarialObjectUUID = owner.AdversarialObjectUUID
	} else {
		it.ImageUUID = owner.ImageUUID
	}
	return it
}

func sortByCreated(items []item) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].CreatedAt != items[j].CreatedAt {
			return items[i].CreatedAt < items[j].CreatedAt
		}
		return items[i].ImageObjectUUID < items[j].ImageObjectUUID
	})
}

type putItem struct {
	it        item
	mustBeNew bool
}

// writeSet is what one transaction writes. An item key is in it only once, as DynamoDB demands.
type writeSet struct {
	puts map[key]putItem
	dels map[key]struct{}
}

func newWriteSet() *writeSet {
	return &writeSet{puts: map[key]putItem{}, dels: map[key]struct{}{}}
}

// put replaces a delete of the same key; mustBeNew is then dropped, as the old item exists until the transaction.
func (w *writeSet) put(it item, mustBeNew bool) {
	k := it.key()
	if _, ok := w.dels[k]; ok {
		delete(w.dels, k)
		mustBeNew = false
	}
	w.puts[k] = putItem{it: it, mustBeNew: mustBeNew}
}

func (w *writeSet) del(k key) {
	if _, ok := w.puts[k]; !ok {
		w.dels[k] = struct{}{}
	}
}

// planPredictions makes the stored ranks of the owner equal to predict: it keeps created_at of the ranks that
// stay and deletes the others.
func planPredictions(w *writeSet, owner item, existing []item, predict []map[string]diagnose_model.ResnetPredictionData, now string) {
	old := make(map[key]item, len(existing))
	for _, e := range existing {
		old[e.key()] = e
	}
	keep := make(map[key]bool, len(predict))
	for rank, p := range predict {
		row := newPredictionItem(owner, rank, p, now, now)
		if o, ok := old[row.key()]; ok && o.CreatedAt != "" {
			row.CreatedAt = o.CreatedAt
		}
		w.put(row, false)
		keep[row.key()] = true
	}
	for k := range old {
		if !keep[k] {
			w.del(k)
		}
	}
}
