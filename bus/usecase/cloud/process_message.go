package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/belobragin/bvv_utils/goutils/abroker"
	"github.com/belobragin/werewolf/bus/internal/pdata"
	"github.com/belobragin/werewolf/bus/pkg/database"
	"github.com/belobragin/werewolf/bus/pkg/model/alter_model"
	"go.uber.org/zap"
)

const processMessageTimeout = 15 * time.Second

// NewProcessMessage saves the ML replies from the sqs queue to the database.
// The message stays in the queue (is redelivered) only if the database failed; a reply that
// can never be saved is logged and dropped.
func NewProcessMessage(db database.ProjectDbaseI, logger *zap.Logger) func(abroker.AllMessageI) error {
	return func(m abroker.AllMessageI) error {
		var reply alter_model.MLMessageReply
		if err := json.Unmarshal(m.GetData(), &reply); err != nil {
			logger.Error("ML reply unmarshal failed", zap.Error(err), zap.ByteString("data", m.GetData()))
			return nil
		}

		data, err := reply.ToAlterPostToDb()
		if errors.Is(err, alter_model.ErrMLFailed) {
			logger.Error("ML service failed to process the order", zap.String("order_id", reply.ImageID), zap.String("error", reply.Error))
			return nil
		}
		if err != nil {
			logger.Error("ML reply is invalid", zap.String("order_id", reply.ImageID), zap.Error(err))
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), processMessageTimeout)
		defer cancel()
		if err = db.DbAlterObject(ctx, data, data.AdversarialObjectID); err != nil {
			if errors.Is(err, pdata.ErrNoObjectId) {
				logger.Error("ML reply is for an unknown image", zap.String("order_id", reply.ImageID), zap.Error(err))
				return nil
			}
			logger.Error("ML reply is not saved to the database", zap.String("order_id", reply.ImageID), zap.Error(err))
			return err
		}

		logger.Info("ML reply is saved",
			zap.String("order_id", reply.ImageID),
			zap.String("algorythm", data.Algorythm),
			zap.String("adversarial_object_id", data.AdversarialObjectID.String()),
			zap.String("adversarial_name", data.AdversarialName),
			zap.Bool("filter", data.HasFilter()),
			zap.Any("predictions", data.Predict),
		)
		return nil
	}
}
