-- 005_fixed_meal_active_label lets an owner re-create a fixed meal after
-- switching it off: the label is unique among active meals only, because a
-- deactivated row stays behind for the plans that still reference it.

ALTER TABLE fixed_meal
    DROP CONSTRAINT fixed_meal_eater_id_meal_type_label_key;

CREATE UNIQUE INDEX fixed_meal_active_label_key
    ON fixed_meal (eater_id, meal_type, label)
    WHERE active;

---- create above / drop below ----

DROP INDEX fixed_meal_active_label_key;

ALTER TABLE fixed_meal
    ADD CONSTRAINT fixed_meal_eater_id_meal_type_label_key UNIQUE (eater_id, meal_type, label);
