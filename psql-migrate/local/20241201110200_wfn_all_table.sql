-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    PERFORM 'wfn.algorythm'::regtype;
    EXCEPTION
    WHEN undefined_object THEN 
        CREATE TYPE wfn.algorythm AS ENUM ('emulator', 'biterate_untargeted', 'biterate_targeted', 'fgsm');
END$$;

create table IF NOT EXISTS wfn.image
(
    image_uuid                                          UUID UNIQUE PRIMARY KEY,
    image_name                                          TEXT UNIQUE NOT NULL,
        CONSTRAINT image_name CHECK (LENGTH(image_name) <= 100),
    image_description                                   TEXT,
        CONSTRAINT image_description CHECK (LENGTH(image_description) <= 2000),
    mime_type                                           TEXT NOT NULL,
    image_size                                          TEXT NOT NULL,
    image_height                                        INTEGER NOT NULL,
    image_width                                         INTEGER NOT NULL,
    created_at                                          TIMESTAMP NOT NULL DEFAULT now()
);

create table IF NOT EXISTS wfn.adversarial_object
(
    adversarial_object_uuid                             UUID UNIQUE PRIMARY KEY,
    adversarial_object_name                             TEXT NOT NULL,
        CONSTRAINT adversarial_object_name CHECK (LENGTH(adversarial_object_name) <= 300),
    image_uuid                                          UUID NOT NULL,
        CONSTRAINT          image_uuid
            FOREIGN KEY     (image_uuid)
            REFERENCES      wfn.image(image_uuid)
            ON DELETE CASCADE,
    with_filter_object                                  BOOL DEFAULT FALSE,
    algorythm                                           wfn.algorythm NOT NULL,
    created_at                                          TIMESTAMP NOT NULL DEFAULT now(),
    alter_goal                                          TEXT,
        CONSTRAINT adversarial_object_alter_goal CHECK (LENGTH(alter_goal) <= 100),
    CONSTRAINT image_uuid_algorythm UNIQUE (image_uuid, algorythm)
);

create table IF NOT EXISTS wfn.filter_object
(
    filter_object_uuid                                  UUID UNIQUE PRIMARY KEY,
    filter_object_name                                  TEXT UNIQUE NOT NULL,
        CONSTRAINT filter_object_name CHECK (LENGTH(filter_object_name) <= 300),
    adversarial_object_uuid                             UUID UNIQUE NOT NULL,
        CONSTRAINT          adversarial_object_uuid
            FOREIGN KEY     (adversarial_object_uuid)
            REFERENCES      wfn.adversarial_object(adversarial_object_uuid)
            ON DELETE CASCADE,
    created_at                                          TIMESTAMP NOT NULL DEFAULT now()
);

create table IF NOT EXISTS wfn.diagnose_result
(
    image_uuid                                          UUID,
        CONSTRAINT          image_uuid
            FOREIGN KEY     (image_uuid)
            REFERENCES      wfn.image(image_uuid)
            ON DELETE CASCADE,
    prediction_rank                                     SMALLINT NOT NULL CHECK (prediction_rank >= 0),
    result                                              JSONB DEFAULT '{}'::jsonb,
    updated_at                                          TIMESTAMP NOT NULL DEFAULT now(),
    created_at                                          TIMESTAMP NOT NULL DEFAULT now(),
    adversarial_object_uuid                             UUID,
        CONSTRAINT          diagnose_result_adversarial_object_uuid
            FOREIGN KEY     (adversarial_object_uuid)
            REFERENCES      wfn.adversarial_object(adversarial_object_uuid)
            ON DELETE CASCADE,
    CONSTRAINT diagnose_result_one_owner CHECK ((image_uuid IS NULL) <> (adversarial_object_uuid IS NULL))
);

CREATE UNIQUE INDEX IF NOT EXISTS diagnose_result_image_rank
    ON wfn.diagnose_result (image_uuid, prediction_rank) WHERE image_uuid IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS diagnose_result_adversarial_rank
    ON wfn.diagnose_result (adversarial_object_uuid, prediction_rank) WHERE adversarial_object_uuid IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS wfn.image CASCADE;
DROP TABLE IF EXISTS wfn.diagnose_result CASCADE;
DROP TABLE IF EXISTS wfn.adversarial_object CASCADE;
DROP TABLE IF EXISTS wfn.filter_object CASCADE;
-- +goose StatementEnd
