package local

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/belobragin/werewolf/bus/pkg/database"
	"github.com/belobragin/werewolf/bus/pkg/model/alter_model"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

const processMessageTimeout = 10 * time.Second

func NewProcessMessage(db database.ProjectDbaseI, logger *zap.Logger) nats.MsgHandler {
	return func(m *nats.Msg) {
		var reply alter_model.MLMessageReply
		if err := json.Unmarshal(m.Data, &reply); err != nil {
			logger.Error("ML reply unmarshal failed", zap.Error(err), zap.ByteString("data", m.Data))
			return
		}

		// makes the new uuids of the adversarial object and of the filter
		data, err := reply.ToAlterPostToDb()
		if errors.Is(err, alter_model.ErrMLFailed) {
			logger.Error("ML service failed to process the order", zap.String("order_id", reply.ImageID), zap.String("error", reply.Error))
			return
		}
		if err != nil {
			logger.Error("ML reply is invalid", zap.String("order_id", reply.ImageID), zap.Error(err))
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), processMessageTimeout)
		defer cancel()
		if err = db.DbAlterObject(ctx, data, data.AdversarialObjectID); err != nil {
			logger.Error("ML reply is not saved to the database", zap.String("order_id", reply.ImageID), zap.Error(err))
			return
		}

		logger.Info("ML reply is saved",
			zap.String("order_id", reply.ImageID),
			zap.String("algorythm", data.Algorythm),
			zap.String("adversarial_object_id", data.AdversarialObjectID.String()),
			zap.String("adversarial_name", data.AdversarialName),
			zap.Bool("filter", data.HasFilter()),
			zap.Any("predictions", data.Predict),
		)
	}
}
