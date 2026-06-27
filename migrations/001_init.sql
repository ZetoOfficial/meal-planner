-- 001_init — начальная схема Meal Planner (tern migration).
-- UP-секция зеркалит docs/schema.sql (канонический DDL для sqlc).
-- Формат tern: всё до разделителя применяется при `migrate`, после — при откате.

-- Meal Planner — Database Schema (ТЗ, MVP v1)
-- Postgres 15+. Источник: docs/DESIGN.md §13 + закрытые открытые вопросы §12.
-- Назначение: type-safe queries через sqlc; OpenAPI-first для хендлеров.
--
-- Принципы:
--   * storage_profile и role-теги — first-class поля, не комментарии (§1, §3).
--   * planned vs actual macros разведены (§12 Q3).
--   * двухпользовательский режим: учёт «кто готовил / кто логировал» (§12 Q4).
--   * история и аналитика заложены заранее (§12 Q5).
--   * usage-based weights НЕ делаем (§12 Q7) — поля под это отсутствуют.
--   * inventory НЕ делаем (§9).

-- =====================================================================
-- ENUMS
-- =====================================================================

-- role_kind — ВЕРХНИЙ уровень роли ингредиента: к какой группе он относится в слоте.
-- Это первая часть role_tag.code ('kind:value:subclass'). Нужен, чтобы слот понимал,
-- продукт какого типа он принимает (слот «белок» берёт только kind=protein и т.д.).
CREATE TYPE role_kind AS ENUM (
    'protein',  -- источник белка: курица, рыба, индейка, говядина, яйца
    'carb',     -- источник углеводов/гарнир: крупы, рис, картофель, паста
    'veg',      -- овощи: брокколи, цукини, перец (объём и клетчатка)
    'sauce',    -- соус/заправка: тахини-дрессинг, томатный, маринад
    'fat',      -- отдельный источник жира: гхи, оливковое масло, авокадо
    'other'     -- всё, что не попадает в категории выше (специи, добавки)
);

-- Подклассы нужны: грудка/бёдра/индейка различаются по БЖУ и стоимости (§12 Q1).
-- Гранулярность хранится в role_tag.code как 'kind:value:subclass'.

-- recipe_status — жизненный цикл рецепта-шаблона в библиотеке (§4 DESIGN).
-- Двухступенчатое наполнение: LLM генерит черновик → после реальной готовки повышаем.
CREATE TYPE recipe_status AS ENUM (
    'draft',     -- сгенерирован LLM и прошёл валидатор, но ещё НЕ готовился вживую;
                 -- в reuse-first НЕ участвует (FR-3)
    'permanent', -- приготовлен и одобрен → переиспользуется в планах без LLM (FR-9)
    'rejected'   -- не понравился; не повышается и не предлагается (бан-листов нет, §4 —
                 -- просто не растёт до permanent)
);

-- recipe_source — кто создал шаблон. Влияет на доверие/логику (нужна ли была валидация LLM-вывода).
CREATE TYPE recipe_source AS ENUM (
    'llm',     -- придуман моделью под constraints (runtime-генерация, §5)
    'manual'   -- заведён руками человеком
);

-- storage_profile — КАК долго блюдо живёт после готовки. Hard constraint §1:
-- хранение в стекле 2–3 дня выкидывает ~60% рецептов, поэтому это first-class поле,
-- а не комментарий. Солвер для плана на 3 дня берёт только первые три значения.
CREATE TYPE storage_profile AS ENUM (
    'keeps_3d',        -- спокойно живёт 2–3 дня в стекле, можно есть холодным
    'keeps_3d_reheat', -- живёт 2–3 дня, но перед едой нужен разогрев
    'assemble_fresh',  -- база (белок/крупа) хранится, но финальная сборка/заправка —
                       -- перед едой (иначе размокнет): салаты, боулы с дрессингом
    'eat_same_day'     -- только в день готовки (корочка, кляр) → для многодневного
                       -- плана почти не годится, но допустимо для готовочного дня
);

-- meal_type — приём пищи в течение дня. Используется и для генерируемых приёмов,
-- и для фиксированных (Сашин завтрак = breakfast).
CREATE TYPE meal_type AS ENUM (
    'breakfast',  -- завтрак
    'lunch',      -- обед
    'dinner',     -- ужин
    'snack'       -- перекус (опционально, для добивки БЖУ)
);

-- plan_status — состояние плана (партии готовки на N дней).
CREATE TYPE plan_status AS ENUM (
    'draft',     -- сгенерирован, но ещё не подтверждён/не закуплен — можно свопать свободно
    'active',    -- принят в работу: по нему готовят и едят
    'archived'   -- прошлый/отменённый план; остаётся в истории для аналитики (§12 Q5)
);

-- meal_log_status — что РЕАЛЬНО произошло с приёмом пищи (planned vs actual, §12 Q3).
-- По умолчанию planned; пользователь меняет руками, только если съел не то/не так.
CREATE TYPE meal_log_status AS ENUM (
    'planned',           -- запланировано, факт ещё не подтверждён
    'eaten_as_planned',  -- съедено как планировалось → в аналитике берём planned-БЖУ
    'eaten_modified',    -- съедено с отклонением → заполнены actual_* поля meal
    'skipped'            -- пропущено/не съедено
);

-- feedback_target — на ЧТО оставлен отзыв. target_id ссылается на запись в
-- соответствующей таблице (полиморфная ссылка). Отзыв двигает вес, не банит (§7).
CREATE TYPE feedback_target AS ENUM (
    'recipe_template', -- оценка рецепта целиком → меняет recipe_template.weight
    'product',         -- оценка конкретного продукта/бренда
    'meal',            -- отзыв о конкретном приёме пищи в плане
    'slot_fill'        -- отзыв о конкретном заполнении слота (напр. «этот рис разварился»)
);

-- llm_call_status — исход обращения к LLM. Нужен для отладки и метрик (§11: логируем ВСЁ).
CREATE TYPE llm_call_status AS ENUM (
    'ok',      -- успешный живой вызов модели
    'error',   -- ошибка/таймаут/невалидный вывод (текст ошибки — в llm_call.error)
    'cached'   -- ответ взят из кэша по хешу промпта, без обращения к провайдеру
);

-- =====================================================================
-- ENUMS — СЛОЙ КАЧЕСТВА ПИТАНИЯ (evidence-based nutrition quality layer)
-- Источник: docs/deep-research-report.md. Идея: поверх ЛИЧНЫХ макро-целей —
-- доказательные правила качества рациона (клетчатка, натрий, насыщенные жиры,
-- сахара, food-group баланс). Главный риск без этого слоя — «macro-correct junk
-- food»: нужный белок/ккал из переработанного мяса, рафинада, сладких десертов.
-- =====================================================================

-- sex — нужен ТОЛЬКО для sex-specific дефолта клетчатки (NNR: 25 ж / 35 м).
-- Не используется ни для чего другого; 'unspecified' → общий дефолт 25 г.
CREATE TYPE sex AS ENUM ('male', 'female', 'unspecified');

-- nutrition_data_quality — насколько можно доверять нутриентам продукта.
-- Критично: unknown ≠ 0. Неизвестная клетчатка НЕ даёт credit; неизвестный
-- натрий НЕ считается безопасным (uncertainty penalty в солвере).
CREATE TYPE nutrition_data_quality AS ENUM (
    'label_exact',  -- точные значения с упаковки
    'trusted_db',   -- из доверенной БД состава продуктов
    'estimated',    -- прикидка/аналог
    'unknown'       -- данных нет
);

-- source_kind — откуда взяты значения нутриентов (provenance).
CREATE TYPE source_kind AS ENUM (
    'pack_label',        -- этикетка упаковки
    'store_card',        -- карточка товара в магазине
    'composition_table', -- таблица химсостава
    'manual_estimate'    -- ручная прикидка
);

-- food_group — пищевая группа для food-based правил (овощи/фрукты, цельные
-- злаки, бобовые, красное/переработанное мясо и т.д.). Отдельно от role_kind:
-- role_kind — про роль в слоте (солвер), food_group — про качество рациона.
CREATE TYPE food_group AS ENUM (
    'fruit', 'vegetable', 'potato',        -- картофель НЕ идёт в зачёт fruit_veg
    'grain', 'legume', 'nuts_seeds',
    'dairy', 'dairy_alt',
    'fish_seafood', 'egg', 'red_meat', 'white_meat', 'processed_meat',
    'mixed', 'sauce_condiment', 'confectionery', 'beverage', 'other'
);

-- processing_level — степень переработки (NOVA-подобно). В MVP: аналитика +
-- мягкий penalty, НЕ hard-gate (NNR сознательно не дал числовой нормы по UPF).
CREATE TYPE processing_level AS ENUM (
    'minimally_processed',
    'processed_culinary',
    'processed_food',
    'ultra_processed',
    'unknown'
);

-- rule_eval_status — ИСХОД проверки правила (результат). Не путать с severity!
CREATE TYPE rule_eval_status AS ENUM (
    'pass',       -- правило соблюдено
    'soft_fail',  -- нарушено (мягко): понижает ранг плана + warning
    'warn',       -- пограничное/предупреждение
    'unknown'     -- не хватает данных для оценки (низкий coverage)
);

-- rule_severity — НАСКОЛЬКО СТРОГО правило (его «тип»), отдельно от исхода.
-- Severity не может быть 'pass'/'unknown' — это про результат, а не про правило.
CREATE TYPE rule_severity AS ENUM (
    'hard',      -- нарушать нельзя (напр. явные транс-жиры)
    'soft',      -- влияет на ранжирование, но не блокирует
    'warn',      -- только предупреждение
    'analytics'  -- не влияет на план, только в статистике
);

-- entity_type — к чему привязан результат оценки правила.
CREATE TYPE rule_entity_type AS ENUM ('meal', 'day', 'rolling_window', 'plan');

-- summary_basis — на чём считается агрегат: план или факт (planned vs actual, §12 Q3).
CREATE TYPE summary_basis AS ENUM ('planned', 'actual');

-- ---- Личные исключения едока (preference bans) ----
-- Отдельно от аллергенов: аллерген — медицинское (продукт СОДЕРЖИТ вещество),
-- исключение — предпочтение («Саша не любит морепродукты»). Исключаем по КАТЕГОРИИ,
-- а не по каждому продукту (рыба+креветки+мидии = одна строка через food_group).

-- exclusion_target — по чему матчим исключение.
CREATE TYPE exclusion_target AS ENUM (
    'food_group',  -- по пищевой группе, напр. 'fish_seafood' (самый частый случай)
    'role_tag',    -- по коду роли, напр. 'protein:lean:thigh'
    'product'      -- по конкретному продукту (id)
);

-- exclusion_severity — насколько строго. hard = никогда не класть; soft = штраф веса.
CREATE TYPE exclusion_severity AS ENUM ('hard', 'soft');

-- =====================================================================
-- ПРОФИЛЬ ЕДОКА  (§13: отдельная сущность, всё крутится вокруг пользователя)
-- =====================================================================

CREATE TABLE eater (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name            TEXT        NOT NULL UNIQUE,            -- 'Паша', 'Саша'
    sex             sex         NOT NULL DEFAULT 'unspecified', -- только для дефолта клетчатки
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Цели по БЖУ — ДИАПАЗОНЫ (min/max) для Б, Ж, У и ккал, с историчностью.
-- Текущая цель едока = строка с максимальным effective_from <= CURRENT_DATE.
-- Старые строки не правятся → история целей сохраняется (аналитика §12 Q5),
-- а смена цели не задним числом меняет уже сгенерированные планы (FR-2).
CREATE TABLE eater_macro_goal (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    eater_id        BIGINT       NOT NULL REFERENCES eater(id) ON DELETE CASCADE,
    effective_from  DATE         NOT NULL DEFAULT CURRENT_DATE,
    protein_min_g   NUMERIC(6,1) NOT NULL,                 -- напр. Паша 135 / Саша 115
    protein_max_g   NUMERIC(6,1) NOT NULL,                 -- напр. Паша 145 / Саша 125
    fat_min_g       NUMERIC(6,1) NOT NULL,                 -- 60 / 55
    fat_max_g       NUMERIC(6,1) NOT NULL,                 -- 70 / 60
    carb_min_g      NUMERIC(6,1) NOT NULL,                 -- 190 / 170
    carb_max_g      NUMERIC(6,1) NOT NULL,                 -- 210 / 190
    kcal_min        INTEGER      NOT NULL,                 -- ~1900 / ~1670
    kcal_max        INTEGER      NOT NULL,                 -- ~2000 / ~1770
    note            TEXT,                                  -- 'старт', 'сушка', причина смены
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT protein_range_ok CHECK (protein_min_g >= 0 AND protein_min_g <= protein_max_g),
    CONSTRAINT fat_range_ok     CHECK (fat_min_g >= 0 AND fat_min_g <= fat_max_g),
    CONSTRAINT carb_range_ok    CHECK (carb_min_g >= 0 AND carb_min_g <= carb_max_g),
    CONSTRAINT kcal_range_ok    CHECK (kcal_min >= 0 AND kcal_min <= kcal_max),
    UNIQUE (eater_id, effective_from)                      -- одна цель на дату начала
);

-- Быстрый поиск текущей/исторической цели.
CREATE INDEX idx_eater_macro_goal_current ON eater_macro_goal(eater_id, effective_from DESC);

-- Аллергены едока (Паша: казеин, орехи, арахис) — МЕДИЦИНСКОЕ исключение.
-- Hard constraint: продукт с пересекающимся аллергеном не попадает в план едока.
CREATE TABLE eater_allergen (
    eater_id    BIGINT NOT NULL REFERENCES eater(id) ON DELETE CASCADE,
    allergen    TEXT   NOT NULL,   -- 'casein', 'nuts', 'peanuts'
    PRIMARY KEY (eater_id, allergen)
);

-- Личные исключения по ПРЕДПОЧТЕНИЯМ (preference bans), напр. «Саша не любит
-- морепродукты». Отдельно от аллергенов (разные семантика, строгость и UX-сообщения).
-- hard → продукт не попадает в план (как явный запрет, SPEC HC-10);
-- soft → штраф веса при выборе (как feedback, DESIGN §7).
CREATE TABLE eater_exclusion (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    eater_id     BIGINT NOT NULL REFERENCES eater(id) ON DELETE CASCADE,
    target_kind  exclusion_target   NOT NULL,
    target_value TEXT               NOT NULL,  -- 'fish_seafood' / 'protein:lean:thigh' / id продукта
    severity     exclusion_severity NOT NULL DEFAULT 'hard',
    note         TEXT,                         -- 'не любит морепродукты'
    created_at   TIMESTAMPTZ        NOT NULL DEFAULT now(),
    UNIQUE (eater_id, target_kind, target_value)
);

-- Быстрый выбор исключений едока в фильтре кандидатов солвера.
CREATE INDEX idx_eater_exclusion_eater ON eater_exclusion(eater_id);

-- Фиксированные приёмы пищи, которые НЕ генерируются, но учитываются в БЖУ дня.
-- Пример: Сашин завтрак — 3 бутерброда (хлеб+ветчина+сыр) каждый день (§1).
CREATE TABLE fixed_meal (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    eater_id    BIGINT      NOT NULL REFERENCES eater(id) ON DELETE CASCADE,
    meal_type   meal_type   NOT NULL,
    label       TEXT        NOT NULL,        -- '3 бутерброда'
    protein_g   NUMERIC(6,1) NOT NULL,
    fat_g       NUMERIC(6,1) NOT NULL,
    carb_g      NUMERIC(6,1) NOT NULL,
    kcal        INTEGER      NOT NULL,
    -- Hot-path нутриенты фикс. приёма — чтобы он НЕ был «невидим» для quality-слоя.
    -- Ветчина = переработанное мясо; фикс-завтрак больше не безусловный baseline
    -- (deep-research-report.md). NULL = неизвестно. Граммаж мяса/злаков/овощей нужен
    -- для rolling-7d недельных правил (boolean недостаточно).
    saturated_fat_g     NUMERIC(6,1),
    trans_fat_g         NUMERIC(6,1),
    fiber_g             NUMERIC(6,1),
    sodium_mg           NUMERIC(8,1),
    added_sugars_g      NUMERIC(6,1),
    free_sugars_g       NUMERIC(6,1),
    processed_meat_g    NUMERIC(7,1),                -- граммаж для rolling-правила
    red_meat_g          NUMERIC(7,1),
    whole_grain_g_equiv NUMERIC(7,1),
    fruit_veg_g         NUMERIC(7,1),
    -- Override-флаги: не генерируется, но участвует в quality-учёте (решение владельца).
    include_in_analytics BOOLEAN NOT NULL DEFAULT TRUE,
    include_in_warnings  BOOLEAN NOT NULL DEFAULT TRUE,
    active      BOOLEAN     NOT NULL DEFAULT TRUE,
    UNIQUE (eater_id, meal_type, label),
    CONSTRAINT fixed_meal_nonneg CHECK (
        protein_g >= 0 AND fat_g >= 0 AND carb_g >= 0 AND kcal >= 0
        AND (saturated_fat_g IS NULL OR saturated_fat_g >= 0)
        AND (trans_fat_g IS NULL OR trans_fat_g >= 0)
        AND (fiber_g IS NULL OR fiber_g >= 0)
        AND (sodium_mg IS NULL OR sodium_mg >= 0)
        AND (added_sugars_g IS NULL OR added_sugars_g >= 0)
        AND (free_sugars_g IS NULL OR free_sugars_g >= 0)
        AND (processed_meat_g IS NULL OR processed_meat_g >= 0)
        AND (red_meat_g IS NULL OR red_meat_g >= 0)
        AND (whole_grain_g_equiv IS NULL OR whole_grain_g_equiv >= 0)
        AND (fruit_veg_g IS NULL OR fruit_veg_g >= 0)
    )
);

-- =====================================================================
-- БИБЛИОТЕКА ПРОДУКТОВ  (личная БД брендов, §4)
-- =====================================================================

CREATE TABLE product (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    brand           TEXT         NOT NULL,           -- 'Троекурово'
    name            TEXT         NOT NULL,           -- 'грудка куриная'
    -- БЖУ на 100 г из реальной упаковки/карточки магазина.
    protein_per_100g NUMERIC(6,2) NOT NULL,
    fat_per_100g     NUMERIC(6,2) NOT NULL,
    carb_per_100g    NUMERIC(6,2) NOT NULL,
    kcal_per_100g    NUMERIC(7,2) NOT NULL,
    -- Стоимость нужна: подклассы белка различаются по цене (§12 Q1).
    price_per_100g  NUMERIC(8,2),                    -- в рублях, NULL если неизвестно
    default_unit    TEXT         NOT NULL DEFAULT 'g', -- 'g', 'ml', 'pcs'
    purchasable     BOOLEAN      NOT NULL DEFAULT TRUE, -- можно ли класть в список покупок
    notes           TEXT,
    -- ---- Hot-path нутриенты для quality-слоя (deep-research-report.md) ----
    -- Все NULLable: NULL = НЕИЗВЕСТНО (не 0!). Солвер трактует unknown как
    -- отсутствие credit для полезных и как uncertainty penalty для рисковых.
    saturated_fat_per_100g NUMERIC(6,2),             -- насыщенные жиры
    trans_fat_per_100g     NUMERIC(6,2),             -- транс-жиры ВСЕГО (может включать
                                                     -- природные из мяса/молока) → только warn
    industrial_trans_fat_per_100g NUMERIC(6,2),      -- именно промышленные → hard reject
    fiber_per_100g         NUMERIC(6,2),             -- клетчатка
    sodium_mg_per_100g     NUMERIC(8,2),             -- натрий (правила считают натрий)
    salt_g_per_100g        NUMERIC(6,2),             -- соль (часто на этикетке РФ);
                                                     -- натрий = соль / 2.54 (NNR)
    total_sugars_per_100g  NUMERIC(6,2),             -- сахара всего
    added_sugars_per_100g  NUMERIC(6,2),             -- добавленные сахара
    free_sugars_per_100g   NUMERIC(6,2),             -- свободные сахара (WHO)
    contains_partially_hydrogenated_oils BOOLEAN,    -- PHO: явный источник транс-жиров
    -- ---- Качество/происхождение данных (unknown ≠ 0) ----
    nutrition_data_quality nutrition_data_quality NOT NULL DEFAULT 'unknown',
    nutrition_coverage_score NUMERIC(4,3),           -- 0..1, доля заполненных hot-path полей
    source_kind     source_kind,                     -- откуда значения
    source_note     TEXT,                            -- сырой текст этикетки / ссылка
    last_verified_at TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (brand, name),
    CONSTRAINT coverage_range CHECK (
        nutrition_coverage_score IS NULL
        OR nutrition_coverage_score BETWEEN 0 AND 1
    ),
    -- Нутриенты/цена не могут быть отрицательными.
    CONSTRAINT product_nonneg CHECK (
        protein_per_100g >= 0 AND fat_per_100g >= 0 AND carb_per_100g >= 0
        AND kcal_per_100g >= 0
        AND (price_per_100g IS NULL OR price_per_100g >= 0)
        AND (saturated_fat_per_100g IS NULL OR saturated_fat_per_100g >= 0)
        AND (trans_fat_per_100g IS NULL OR trans_fat_per_100g >= 0)
        AND (industrial_trans_fat_per_100g IS NULL OR industrial_trans_fat_per_100g >= 0)
        AND (fiber_per_100g IS NULL OR fiber_per_100g >= 0)
        AND (sodium_mg_per_100g IS NULL OR sodium_mg_per_100g >= 0)
        AND (salt_g_per_100g IS NULL OR salt_g_per_100g >= 0)
        AND (total_sugars_per_100g IS NULL OR total_sugars_per_100g >= 0)
        AND (added_sugars_per_100g IS NULL OR added_sugars_per_100g >= 0)
        AND (free_sugars_per_100g IS NULL OR free_sugars_per_100g >= 0)
    )
);

-- Пищевая классификация продукта — отдельно от нутриентов и от role_kind.
-- Нужна для food-based правил (овощи/фрукты, цельные злаки, бобовые, мясо).
-- Counting semantics хранятся явно, не выводятся из food_group на лету:
-- картофель НЕ в fruit_veg, сок НЕ как цельный фрукт (deep-research-report.md).
CREATE TABLE product_food_classification (
    product_id          BIGINT  PRIMARY KEY REFERENCES product(id) ON DELETE CASCADE,
    food_group          food_group       NOT NULL DEFAULT 'other',
    processing_level    processing_level NOT NULL DEFAULT 'unknown',
    processing_confidence NUMERIC(4,3),              -- 0..1, уверенность в классе
    counts_to_fruit_veg BOOLEAN NOT NULL DEFAULT FALSE, -- идёт ли в зачёт ≥400 г/день
    is_legume           BOOLEAN NOT NULL DEFAULT FALSE,
    is_whole_grain      BOOLEAN NOT NULL DEFAULT FALSE,
    whole_grain_ratio   NUMERIC(4,3),               -- доля цельнозерновой части 0..1
    is_red_meat         BOOLEAN NOT NULL DEFAULT FALSE,
    is_processed_meat   BOOLEAN NOT NULL DEFAULT FALSE,
    classification_source TEXT,
    classification_note TEXT,
    CONSTRAINT proc_conf_range CHECK (
        processing_confidence IS NULL OR processing_confidence BETWEEN 0 AND 1),
    CONSTRAINT wg_ratio_range CHECK (
        whole_grain_ratio IS NULL OR whole_grain_ratio BETWEEN 0 AND 1)
);

-- Аллергены, содержащиеся в продукте (для пересечения с eater_allergen).
CREATE TABLE product_allergen (
    product_id  BIGINT NOT NULL REFERENCES product(id) ON DELETE CASCADE,
    allergen    TEXT   NOT NULL,    -- 'casein', 'nuts', 'peanuts'
    PRIMARY KEY (product_id, allergen)
);

-- Справочник ролевых тегов с подклассами (§12 Q1).
-- code = 'kind:value[:subclass]', напр. 'protein:lean:breast', 'protein:lean:thigh',
-- 'carb:grain', 'veg:green', 'sauce:fat'.
CREATE TABLE role_tag (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code        TEXT      NOT NULL UNIQUE,
    kind        role_kind NOT NULL,
    label       TEXT      NOT NULL,         -- человекочитаемо: 'постный белок: грудка'
    parent_id   BIGINT    REFERENCES role_tag(id) ON DELETE SET NULL  -- иерархия класс→подкласс
);

-- Связь продукт ↔ роли (продукт может играть несколько ролей).
CREATE TABLE product_role (
    product_id  BIGINT NOT NULL REFERENCES product(id) ON DELETE CASCADE,
    role_tag_id BIGINT NOT NULL REFERENCES role_tag(id) ON DELETE CASCADE,
    PRIMARY KEY (product_id, role_tag_id)
);

-- =====================================================================
-- БИБЛИОТЕКА РЕЦЕПТОВ-ШАБЛОНОВ  (§3, §4)
-- =====================================================================

-- Рецепт — шаблон со слотами, а не фиксированный список ингредиентов (§3).
CREATE TABLE recipe_template (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name            TEXT            NOT NULL,
    status          recipe_status   NOT NULL DEFAULT 'draft', -- draft→permanent (§4)
    source          recipe_source   NOT NULL DEFAULT 'llm',
    cuisine         TEXT,                                     -- 'азиатская', 'средиземноморская'
    cooking_method  TEXT,                                     -- 'духовка', 'аэрогриль'
    storage         storage_profile NOT NULL,                 -- first-class (§1)
    storage_notes   TEXT,
    equipment       TEXT[]          NOT NULL DEFAULT '{}',    -- ['духовка','аэрогриль']
    instructions    TEXT,                                     -- шаги готовки
    -- Вес для weighted random ротации и «удиви меня» (§7). Feedback меняет его (§7),
    -- а НЕ банит. Дефолт 1.0; >1 чаще, <1 реже; 0 фактически не выпадает.
    weight          NUMERIC(5,3)    NOT NULL DEFAULT 1.0,
    cooked_count    INTEGER         NOT NULL DEFAULT 0,       -- сколько раз реально готовили
    promoted_at     TIMESTAMPTZ,                              -- момент повышения до permanent
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    CONSTRAINT weight_nonneg CHECK (weight >= 0)
);

-- Слот шаблона: ловит КЛАСС по тегу, заполняется конкретным продуктом (§3).
CREATE TABLE recipe_slot (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    template_id BIGINT      NOT NULL REFERENCES recipe_template(id) ON DELETE CASCADE,
    role_tag_id BIGINT      NOT NULL REFERENCES role_tag(id),  -- какой класс принимает слот
    label       TEXT        NOT NULL,           -- 'белок', 'крупа', 'овощ', 'соус'
    qty         NUMERIC(7,1) NOT NULL,          -- 180, 200, 30
    unit        TEXT        NOT NULL DEFAULT 'g', -- 'g', 'ml'
    position    INTEGER     NOT NULL DEFAULT 0,
    optional    BOOLEAN     NOT NULL DEFAULT FALSE,
    UNIQUE (template_id, position),
    CONSTRAINT recipe_slot_qty_positive CHECK (qty > 0)
);

-- =====================================================================
-- ПЛАН  (§6, §8)
-- =====================================================================

-- План = партия готовки на N дней (сессия). Неделя = 2 сессии (§8).
CREATE TABLE plan (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    label           TEXT,                              -- 'Сессия 1 (вс)'
    start_date      DATE        NOT NULL,
    days            INTEGER     NOT NULL DEFAULT 3,    -- 3 дня на сессию
    cook_date       DATE        NOT NULL,             -- готовочный день = первый день еды
    novelty_level   SMALLINT    NOT NULL DEFAULT 0,   -- dial новизны 0–3 (§7)
    status          plan_status NOT NULL DEFAULT 'draft',
    -- Quality-слой: план не блокируется за нарушение quality-правил, но
    -- получает score и warnings (решение владельца: ranking + warnings).
    quality_score   NUMERIC(5,2),                     -- агрегатная оценка качества плана
    quality_warnings_count INTEGER NOT NULL DEFAULT 0,
    -- Список покупок устарел после свопа, изменившего состав продуктов (FR-5).
    shopping_list_stale BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT days_positive CHECK (days > 0),
    CONSTRAINT novelty_range CHECK (novelty_level BETWEEN 0 AND 3)
);

-- Экземпляр рецепта внутри плана: шаблон + конкретные заполнения слотов.
-- Это «компонент» партии (2 белка + 2 гарнира + овощи + соусы → комбинаторика).
CREATE TABLE plan_recipe (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    plan_id     BIGINT  NOT NULL REFERENCES plan(id) ON DELETE CASCADE,
    template_id BIGINT  NOT NULL REFERENCES recipe_template(id),
    label       TEXT,
    batch_qty   NUMERIC(8,1),     -- сколько готовим всего (на все порции партии)
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT plan_recipe_batch_qty_nonneg CHECK (batch_qty IS NULL OR batch_qty >= 0)
);

-- Заполнение слота: какой Product встал в слот этого экземпляра (§3, §13 SlotFill).
-- Своп = смена product_id здесь — основная операция системы (§6).
CREATE TABLE slot_fill (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    plan_recipe_id  BIGINT  NOT NULL REFERENCES plan_recipe(id) ON DELETE CASCADE,
    slot_id         BIGINT  NOT NULL REFERENCES recipe_slot(id),
    product_id      BIGINT  NOT NULL REFERENCES product(id),
    qty_g           NUMERIC(7,1) NOT NULL,    -- фактическое количество в этом заполнении
    UNIQUE (plan_recipe_id, slot_id),
    CONSTRAINT slot_fill_qty_positive CHECK (qty_g > 0)
);

-- Prep dependency graph (§6): блюдо B использует «вчерашнюю курицу» из блюда A.
-- Без графа свопы ломают план тихо. Своп source_recipe ⇒ dependent_recipe невалиден.
CREATE TABLE prep_dependency (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    plan_id             BIGINT NOT NULL REFERENCES plan(id) ON DELETE CASCADE,
    dependent_recipe_id BIGINT NOT NULL REFERENCES plan_recipe(id) ON DELETE CASCADE,
    source_recipe_id    BIGINT NOT NULL REFERENCES plan_recipe(id) ON DELETE CASCADE,
    note                TEXT,             -- 'использует запечённую курицу из обеда дня 1'
    CONSTRAINT no_self_dep CHECK (dependent_recipe_id <> source_recipe_id),
    UNIQUE (dependent_recipe_id, source_recipe_id)
);

-- =====================================================================
-- ПРИЁМЫ ПИЩИ И ПОРЦИИ  (planned vs actual, §12 Q3, Q4)
-- =====================================================================

-- Конкретный приём пищи для конкретного едока в конкретный день.
-- planned_* — рассчитано солвером; actual_* — пользователь пишет руками,
-- только если съел не то / не так (§12 Q3). NULL actual = ел как запланировано.
CREATE TABLE meal (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    plan_id         BIGINT      NOT NULL REFERENCES plan(id) ON DELETE CASCADE,
    eater_id        BIGINT      NOT NULL REFERENCES eater(id),
    meal_date       DATE        NOT NULL,
    meal_type       meal_type   NOT NULL,
    -- Источник еды: либо экземпляр рецепта из плана, либо фиксированный приём.
    plan_recipe_id  BIGINT      REFERENCES plan_recipe(id) ON DELETE SET NULL,
    fixed_meal_id   BIGINT      REFERENCES fixed_meal(id),
    -- Planned macros (детерминированный расчёт солвера).
    planned_protein_g NUMERIC(6,1) NOT NULL DEFAULT 0,
    planned_fat_g     NUMERIC(6,1) NOT NULL DEFAULT 0,
    planned_carb_g    NUMERIC(6,1) NOT NULL DEFAULT 0,
    planned_kcal      INTEGER      NOT NULL DEFAULT 0,
    -- Actual macros (опционально, заполняется при отклонении).
    actual_protein_g  NUMERIC(6,1),
    actual_fat_g      NUMERIC(6,1),
    actual_carb_g     NUMERIC(6,1),
    actual_kcal       INTEGER,
    -- Двухпользовательский учёт (§12 Q4): кто готовил, кто логировал.
    cooked_by       BIGINT      REFERENCES eater(id),
    cooked_at       TIMESTAMPTZ,
    logged_by       BIGINT      REFERENCES eater(id),
    logged_at       TIMESTAMPTZ,
    log_status      meal_log_status NOT NULL DEFAULT 'planned',
    -- Ровно один источник: либо рецепт плана, либо фиксированный приём.
    CONSTRAINT one_source CHECK (
        (plan_recipe_id IS NOT NULL)::int + (fixed_meal_id IS NOT NULL)::int = 1
    ),
    CONSTRAINT meal_nonneg CHECK (
        planned_protein_g >= 0 AND planned_fat_g >= 0 AND planned_carb_g >= 0
        AND planned_kcal >= 0
        AND (actual_protein_g IS NULL OR actual_protein_g >= 0)
        AND (actual_fat_g IS NULL OR actual_fat_g >= 0)
        AND (actual_carb_g IS NULL OR actual_carb_g >= 0)
        AND (actual_kcal IS NULL OR actual_kcal >= 0)
    )
);

-- =====================================================================
-- FEEDBACK  (§7: вес, а не чёрный список)
-- =====================================================================

CREATE TABLE feedback_event (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    eater_id    BIGINT          NOT NULL REFERENCES eater(id),
    target_type feedback_target NOT NULL,
    target_id   BIGINT          NOT NULL,   -- id рецепта/продукта/meal/slot_fill
    rating      SMALLINT,                   -- -2..+2, опционально
    comment     TEXT,                       -- свободный текст; в v2 парсится LLM (§5)
    weight_delta NUMERIC(5,3),              -- как сдвинуть вероятность (§7), не бан
    created_at  TIMESTAMPTZ     NOT NULL DEFAULT now(),
    CONSTRAINT rating_range CHECK (rating IS NULL OR rating BETWEEN -2 AND 2)
);

-- =====================================================================
-- СПИСОК ПОКУПОК  (производная от плана, §2, §13)
-- =====================================================================

CREATE TABLE shopping_list (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    plan_id     BIGINT      NOT NULL REFERENCES plan(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (plan_id)
);

CREATE TABLE shopping_list_item (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    list_id         BIGINT  NOT NULL REFERENCES shopping_list(id) ON DELETE CASCADE,
    product_id      BIGINT  NOT NULL REFERENCES product(id),
    total_qty_g     NUMERIC(9,1) NOT NULL,
    unit            TEXT    NOT NULL DEFAULT 'g',
    est_cost        NUMERIC(9,2),       -- из product.price_per_100g
    checked         BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE (list_id, product_id),
    CONSTRAINT shopping_item_nonneg CHECK (
        total_qty_g >= 0 AND (est_cost IS NULL OR est_cost >= 0))
);

-- =====================================================================
-- LLM-ЛОГ И КЭШ  (§11: логировать ВСЁ; кэш по хешу промпта)
-- =====================================================================

CREATE TABLE llm_call (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    plan_id     BIGINT          REFERENCES plan(id) ON DELETE SET NULL,
    provider    TEXT            NOT NULL,        -- 'anthropic', 'openai'
    model       TEXT            NOT NULL,        -- 'claude-sonnet-4-6'
    temperature NUMERIC(3,2)    NOT NULL,        -- dial новизны на уровне API (§7, §11)
    prompt_hash TEXT            NOT NULL,        -- sha256 для кэша
    prompt      TEXT            NOT NULL,
    response    TEXT,
    tokens_in   INTEGER,
    tokens_out  INTEGER,
    latency_ms  INTEGER,
    status      llm_call_status NOT NULL DEFAULT 'ok',
    error       TEXT,                            -- graceful fail логируется (§12 Q6)
    created_at  TIMESTAMPTZ     NOT NULL DEFAULT now()
);

-- Кэш ответов по хешу промпта (Postgres достаточно, §11).
CREATE TABLE llm_cache (
    prompt_hash TEXT        PRIMARY KEY,
    model       TEXT        NOT NULL,
    temperature NUMERIC(3,2) NOT NULL,
    response    TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- =====================================================================
-- QUALITY-СЛОЙ: профиль качества, агрегаты, результаты правил
-- (deep-research-report.md). Сами ОПРЕДЕЛЕНИЯ правил живут в конфиге/коде
-- (прагматичный MVP без data-driven guideline-движка), а здесь — привязка
-- набора правил к едоку, его override'ы, посчитанные агрегаты и результаты.
-- =====================================================================

-- Какой набор правил действует для едока + его персональные отклонения.
-- guideline_set_code — строковый код набора из конфига (напр. 'who_nnr_dga_2025').
CREATE TABLE eater_quality_profile (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    eater_id            BIGINT  NOT NULL REFERENCES eater(id) ON DELETE CASCADE,
    guideline_set_code  TEXT    NOT NULL DEFAULT 'who_nnr_dga_2025',
    active_from         DATE    NOT NULL DEFAULT CURRENT_DATE,
    note                TEXT,
    UNIQUE (eater_id, active_from)
);

-- Персональный override отдельного правила (вкл/выкл, своя суровость/порог).
-- rule_code — код правила из конфига (напр. 'sodium_max_daily_mg').
-- ВНИМАНИЕ: в MVP это v2/internal — НЕ выставляется в публичный API. Правила
-- задаются в конфиге; таблица оставлена под будущие per-eater override'ы.
-- severity_override использует rule_severity (тип правила), НЕ rule_eval_status.
CREATE TABLE eater_rule_override (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    eater_id        BIGINT  NOT NULL REFERENCES eater(id) ON DELETE CASCADE,
    rule_code       TEXT    NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    severity_override rule_severity,                 -- NULL = дефолт правила
    threshold_override NUMERIC(10,2),                -- NULL = дефолт правила
    rationale       TEXT,
    active_from     DATE    NOT NULL DEFAULT CURRENT_DATE,
    UNIQUE (eater_id, rule_code, active_from)
);

-- Дневной агрегат нутриентов/качества по едоку. Planned и actual — разными
-- строками (basis). Считается из slot_fill×product + fixed_meal. Нужен, чтобы
-- проверять дневные правила (клетчатка, натрий, sat fat, добавл. сахар).
CREATE TABLE day_nutrition_summary (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    eater_id        BIGINT      NOT NULL REFERENCES eater(id) ON DELETE CASCADE,
    plan_id         BIGINT      REFERENCES plan(id) ON DELETE SET NULL,
    summary_date    DATE        NOT NULL,
    basis           summary_basis NOT NULL,
    -- макросы (для полноты картины дня вместе с качеством)
    protein_g       NUMERIC(7,1),
    fat_g           NUMERIC(7,1),
    carb_g          NUMERIC(7,1),
    kcal            INTEGER,
    -- hot-path качество
    saturated_fat_g NUMERIC(7,1),
    trans_fat_g     NUMERIC(7,1),
    fiber_g         NUMERIC(7,1),
    sodium_mg       NUMERIC(9,1),
    added_sugars_g  NUMERIC(7,1),
    free_sugars_g   NUMERIC(7,1),
    -- food-group объёмы
    fruit_veg_g     NUMERIC(8,1),                   -- без картофеля и соков
    whole_grain_g_equiv NUMERIC(8,1),
    legume_g        NUMERIC(8,1),
    red_meat_g      NUMERIC(8,1),
    processed_meat_g NUMERIC(8,1),
    upf_kcal_share_est NUMERIC(4,3),                -- 0..1, доля ккал из UPF (оценка)
    data_coverage_score NUMERIC(4,3),              -- 0..1, насколько данным можно верить
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (eater_id, summary_date, basis),
    CONSTRAINT day_summary_shares CHECK (
        (upf_kcal_share_est  IS NULL OR upf_kcal_share_est  BETWEEN 0 AND 1)
        AND (data_coverage_score IS NULL OR data_coverage_score BETWEEN 0 AND 1)
    ),
    CONSTRAINT day_summary_nonneg CHECK (
        COALESCE(protein_g,0) >= 0 AND COALESCE(fat_g,0) >= 0 AND COALESCE(carb_g,0) >= 0
        AND COALESCE(kcal,0) >= 0 AND COALESCE(saturated_fat_g,0) >= 0
        AND COALESCE(trans_fat_g,0) >= 0 AND COALESCE(fiber_g,0) >= 0
        AND COALESCE(sodium_mg,0) >= 0 AND COALESCE(added_sugars_g,0) >= 0
        AND COALESCE(free_sugars_g,0) >= 0 AND COALESCE(fruit_veg_g,0) >= 0
        AND COALESCE(whole_grain_g_equiv,0) >= 0 AND COALESCE(legume_g,0) >= 0
        AND COALESCE(red_meat_g,0) >= 0 AND COALESCE(processed_meat_g,0) >= 0
    )
);

-- Скользящее окно (3/7/14 дней) для НЕДЕЛЬНЫХ правил: красное/переработанное
-- мясо, бобовые. Позволяет оценивать weekly-правила даже при горизонте плана 3 дня.
CREATE TABLE rolling_nutrition_summary (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    eater_id        BIGINT      NOT NULL REFERENCES eater(id) ON DELETE CASCADE,
    as_of_date      DATE        NOT NULL,
    window_days     SMALLINT    NOT NULL,           -- 3 / 7 / 14
    basis           summary_basis NOT NULL,
    red_meat_g      NUMERIC(9,1),                   -- сумма за окно (правило ≤350 г/нед)
    processed_meat_g NUMERIC(9,1),                  -- сумма за окно (минимизировать)
    legume_servings NUMERIC(6,1),                   -- порций за окно (цель ≥3/нед)
    fiber_g_avg     NUMERIC(7,1),                   -- среднее в день
    sodium_mg_avg   NUMERIC(9,1),
    saturated_fat_g_avg NUMERIC(7,1),
    added_sugars_g_avg NUMERIC(7,1),
    fruit_veg_g_avg NUMERIC(8,1),
    data_coverage_score NUMERIC(4,3),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (eater_id, as_of_date, window_days, basis),
    CONSTRAINT rolling_window_ok CHECK (window_days > 0),
    CONSTRAINT rolling_coverage_ok CHECK (
        data_coverage_score IS NULL OR data_coverage_score BETWEEN 0 AND 1),
    CONSTRAINT rolling_nonneg CHECK (
        COALESCE(red_meat_g,0) >= 0 AND COALESCE(processed_meat_g,0) >= 0
        AND COALESCE(legume_servings,0) >= 0 AND COALESCE(fiber_g_avg,0) >= 0
        AND COALESCE(sodium_mg_avg,0) >= 0 AND COALESCE(saturated_fat_g_avg,0) >= 0
        AND COALESCE(added_sugars_g_avg,0) >= 0 AND COALESCE(fruit_veg_g_avg,0) >= 0
    )
);

-- Результат проверки одного правила для сущности (день/окно/план/приём).
-- Даёт explainability: пользователь видит ПОЧЕМУ план хорош или где компромисс.
-- rule_code ссылается на правило из конфига (не FK — движок правил в коде).
CREATE TABLE rule_evaluation_result (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    entity_type     rule_entity_type NOT NULL,
    entity_id       BIGINT      NOT NULL,           -- id day/rolling/plan/meal
    eater_id        BIGINT      REFERENCES eater(id) ON DELETE CASCADE,
    rule_code       TEXT        NOT NULL,
    status          rule_eval_status NOT NULL,
    actual_value    NUMERIC(12,2),
    target_value    NUMERIC(12,2),
    unit            TEXT,                           -- 'g', 'mg', '% kcal', 'g/week'
    -- Привязка по времени: для какого дня (день) или окна (rolling). План на двоих,
    -- поэтому warning должен быть понятно адресован: кому (eater_id), за что (date/окно).
    eval_date       DATE,                           -- для entity_type='day'/'meal'
    window_start    DATE,                           -- для entity_type='rolling_window'
    window_end      DATE,
    coverage_score  NUMERIC(4,3),                   -- доверие к этой оценке (unknown ≠ 0)
    message         TEXT,                           -- человекочитаемое объяснение/warning
    evaluated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- =====================================================================
-- ИНДЕКСЫ  (под частые запросы солвера, ротации, аналитики §12 Q5)
-- =====================================================================

CREATE INDEX idx_product_role_role         ON product_role(role_tag_id);
CREATE INDEX idx_recipe_template_status    ON recipe_template(status);
CREATE INDEX idx_recipe_slot_template      ON recipe_slot(template_id);
CREATE INDEX idx_slot_fill_product         ON slot_fill(product_id);
CREATE INDEX idx_plan_recipe_plan          ON plan_recipe(plan_id);
CREATE INDEX idx_meal_plan                 ON meal(plan_id);
CREATE INDEX idx_meal_eater_date           ON meal(eater_id, meal_date);   -- аналитика по неделям
CREATE INDEX idx_meal_recipe               ON meal(plan_recipe_id);
CREATE INDEX idx_feedback_target           ON feedback_event(target_type, target_id);
CREATE INDEX idx_llm_call_plan             ON llm_call(plan_id);
CREATE INDEX idx_llm_call_created          ON llm_call(created_at);
-- quality-слой
CREATE INDEX idx_pfc_food_group            ON product_food_classification(food_group);
CREATE INDEX idx_day_summary_eater_date    ON day_nutrition_summary(eater_id, summary_date);
CREATE INDEX idx_rolling_eater             ON rolling_nutrition_summary(eater_id, as_of_date, window_days);
CREATE INDEX idx_rule_eval_entity          ON rule_evaluation_result(entity_type, entity_id);
CREATE INDEX idx_rule_eval_eater           ON rule_evaluation_result(eater_id);

---- create above / drop below ----

-- Откат: таблицы (CASCADE снимает индексы/FK), затем enum-типы.
DROP TABLE IF EXISTS rule_evaluation_result CASCADE;
DROP TABLE IF EXISTS rolling_nutrition_summary CASCADE;
DROP TABLE IF EXISTS day_nutrition_summary CASCADE;
DROP TABLE IF EXISTS eater_rule_override CASCADE;
DROP TABLE IF EXISTS eater_quality_profile CASCADE;
DROP TABLE IF EXISTS llm_cache CASCADE;
DROP TABLE IF EXISTS llm_call CASCADE;
DROP TABLE IF EXISTS shopping_list_item CASCADE;
DROP TABLE IF EXISTS shopping_list CASCADE;
DROP TABLE IF EXISTS feedback_event CASCADE;
DROP TABLE IF EXISTS meal CASCADE;
DROP TABLE IF EXISTS prep_dependency CASCADE;
DROP TABLE IF EXISTS slot_fill CASCADE;
DROP TABLE IF EXISTS plan_recipe CASCADE;
DROP TABLE IF EXISTS plan CASCADE;
DROP TABLE IF EXISTS recipe_slot CASCADE;
DROP TABLE IF EXISTS recipe_template CASCADE;
DROP TABLE IF EXISTS product_role CASCADE;
DROP TABLE IF EXISTS role_tag CASCADE;
DROP TABLE IF EXISTS product_allergen CASCADE;
DROP TABLE IF EXISTS product_food_classification CASCADE;
DROP TABLE IF EXISTS product CASCADE;
DROP TABLE IF EXISTS fixed_meal CASCADE;
DROP TABLE IF EXISTS eater_exclusion CASCADE;
DROP TABLE IF EXISTS eater_allergen CASCADE;
DROP TABLE IF EXISTS eater_macro_goal CASCADE;
DROP TABLE IF EXISTS eater CASCADE;

DROP TYPE IF EXISTS exclusion_severity;
DROP TYPE IF EXISTS exclusion_target;
DROP TYPE IF EXISTS summary_basis;
DROP TYPE IF EXISTS rule_entity_type;
DROP TYPE IF EXISTS rule_severity;
DROP TYPE IF EXISTS rule_eval_status;
DROP TYPE IF EXISTS processing_level;
DROP TYPE IF EXISTS food_group;
DROP TYPE IF EXISTS source_kind;
DROP TYPE IF EXISTS nutrition_data_quality;
DROP TYPE IF EXISTS sex;
DROP TYPE IF EXISTS llm_call_status;
DROP TYPE IF EXISTS feedback_target;
DROP TYPE IF EXISTS meal_log_status;
DROP TYPE IF EXISTS plan_status;
DROP TYPE IF EXISTS meal_type;
DROP TYPE IF EXISTS storage_profile;
DROP TYPE IF EXISTS recipe_source;
DROP TYPE IF EXISTS recipe_status;
DROP TYPE IF EXISTS role_kind;
