# Meal Planner — ТЗ (MVP v1)

> Производное от `docs/DESIGN.md`. Дизайн зафиксирован, открытые вопросы (§12 DESIGN) закрыты.
> Скоуп v1: owner-only household planner для произвольного непустого набора едоков, **HTTP API** (бот и Mini App — отдельный проект, v2).
> Артефакты ТЗ: этот файл (функциональные требования) + `docs/schema.sql` (схема БД) + `api/openapi/api/api.yaml` (контракт API, схемы — `api/openapi/model/api.yaml`).

---

## 1. Назначение и границы

Система планирует домашнюю компонентную готовку на 2–3 дня под цели по БЖУ **и поверх них — под evidence-based качество рациона** (клетчатка, натрий, насыщенные жиры, сахара, доля овощей/фруктов, цельнозерновость, бобовые, ограничение красного/переработанного мяса), с hard-constraints (аллергии, хранение в стекле, оборудование, ротация). Закрывает ежедневное «что готовить», не даёт переедать **и не даёт собрать «macro-correct junk food»** — формально верный по БЖУ, но плохой по качеству рацион (см. `docs/deep-research-report.md`).

Ключевое разделение (не смешивать в одну сущность/score): **личные макро-цели** едока (что человек хочет) ≠ **доказательные правила качества** (что делает рацион здоровым). Первые — hard и персональные, вторые — преимущественно soft с предупреждениями.

**В скоупе v1:**

1. Ручной CRUD продуктов в личную БД (реальные БЖУ брендов, теги-роли, стоимость, **optional nutrition-quality поля + food-классификация + метаданные качества данных**).
2. Генерация плана на 3 дня под constraints (reuse-first: сперва постоянные рецепты, LLM — при отсутствии допустимого permanent-only плана или явном `novelty_level = 3`), **учитывающая quality-правила поверх макро-целей**.
3. Свопы компонентов с показом blast radius **до** подтверждения (**+ diff качества**).
4. Список покупок как производная от плана.
5. Логирование факта еды (planned vs actual) с учётом «кто готовил / кто логировал».
6. Feedback как вес (понижает/повышает вероятность), не чёрный список.
7. История и аналитика: **macro compliance + quality compliance + coverage quality**.

**Вне скоупа v1 (см. DESIGN §2, §9):** Telegram-бот/Mini App, голос, vision/штрихкод, FatSecret-синк, нотификации, инвентарь холодильника, usage-based weights, RAG/векторные БД, agentic frameworks, fine-tuning, тренировки/вода/добавки/HealthKit/sharing.

---

## 2. Акторы

| Актор | Описание |
|---|---|
| Едок (Eater) | Персональный профиль: версионируемые цели БЖУ, аллергены, исключения, фиксированные приёмы, `sex` (только для дефолта клетчатки), quality-профиль. Паша и Саша — стартовые примеры, не ограничение системы. |
| Клиент API | HTTP-клиент v1 (curl/Postman/будущий бот). Auth в owner-only — минимальный (см. §9). |
| LLM-провайдер | Внешний stateless эндпоинт. Единственная активная точка в MVP — генерация рецептов. |

Профили едоков на старте (из DESIGN §1). Цели по БЖУ задаются **диапазонами** (min–max) для Б, Ж, У и ккал и **версионируются** — каждая правка создаёт новую запись с `effective_from`, старые сохраняются (история целей):

| | Белки | Жиры | Углеводы | ккал | Аллергены (медицина) | Исключения (предпочтения) | Фикс. приём |
|---|---|---|---|---|---|---|---|
| Паша | 135–145 г | 60–70 г | 190–210 г | 1900–2000 | казеин, орехи, арахис | — | — |
| Саша | 115–125 г | 55–60 г | 170–190 г | 1670–1770 | — | морепродукты (рыба, креветки и т.п.) | завтрак: 3 бутерброда (хлеб+ветчина+сыр) |

> Аллергены и исключения — разные сущности: аллерген медицинский (продукт *содержит* вещество), исключение — личный запрет по предпочтению. Подробнее — §3.1 (HC-10).

> Числа диапазонов — стартовый сид вокруг ориентиров из DESIGN §1; редактируются через API.

---

## 3. Доменные правила

Все правила проверяются **детерминированно** (солвер/валидатор), не LLM. Делятся на две группы: **hard feasibility & safety** (нарушать нельзя) и **nutrition quality rules** (преимущественно soft: понижают ранг плана + предупреждают, но не блокируют).

### 3.1 Hard constraints (feasibility & safety)

- **HC-1 Аллергены.** Продукт, чьи аллергены пересекаются с `eater_allergen` едока, не попадает в его план. Казеин Паши исключает молоко, творог, сыр, йогурт, сметану, обычное сливочное масло (гхи допустим). Сыр/ветчина — только в Сашином фиксированном завтраке.
- **HC-2 Орехи/арахис** исключены полностью у Паши. Тахини (кунжут) разрешён.
- **HC-3 Хранение.** `storage_profile` рецепта — first-class поле. План на 3 дня берёт только рецепты с профилем `keeps_3d` / `keeps_3d_reheat` / `assemble_fresh`. `eat_same_day` в многодневный план не ставится.
- **HC-4 Оборудование.** Рецепт не используется, если его `equipment` не покрывается доступным набором (плита, аэрогриль, духовка + опционально докупленное).
- **HC-5 Фиксированные приёмы.** `fixed_meal` не генерируется, но его БЖУ всегда учитывается в дневном балансе едока **и в quality-аналитике/предупреждениях** (флаги `include_in_analytics` / `include_in_warnings`). Для полной реконструкции food-based правил он хранит nullable prepared-equivalent `grain_g_equiv`, `whole_grain_g_equiv`, `legume_g`, meat/fruit-veg grams и `upf_kcal`; NULL остаётся unknown.
- **HC-6 Целостность слотов.** Каждый продукт в `slot_fill` обязан иметь роль (`product_role`), совпадающую с `role_tag` слота. Продукт не из БД в план не попадает. Nutrition-значимые количества рецепта и плана нормализуются в граммы; без явной конверсии solver не угадывает вес `ml` или `pcs`.
- **HC-7 Окно готовки.** Между двумя готовками не больше 3–4 дней; партия живёт 2–3 дня (DESIGN §8).
- **HC-8 Персональные макро-цели** — hard: дневная сумма Б/Ж/У/ккал должна попасть в диапазоны `eater_macro_goal`, действующей на конкретную дату еды. Версия goal, использованная для каждого eater/day, сохраняется с планом.
- **HC-9 Промышленные транс-жиры** — hard: продукт не предлагается в генерации, если `contains_partially_hydrogenated_oils = true` **или** `industrial_trans_fat_per_100g > 0`. Поле `trans_fat_per_100g` (всего, может включать природные из мяса/молока) к hard-reject НЕ приводит — только warn.
- **HC-10 Личные исключения (preference bans).** Едок может явно запретить категорию/роль/продукт (`eater_exclusion`), напр. Саша — морепродукты (`food_group = fish_seafood`). При `severity = hard` совпавший продукт не попадает в план едока (как аллерген, но это предпочтение, а не медицина). Исключают по **категории**, а не перечислением брендов (рыба + креветки + мидии — одна запись). При `severity = soft` это не hard-constraint, а штраф веса (см. §3.3).
- **HC-11 Целостность партии.** Generated meal состоит из одного или нескольких `meal_component_allocation`. Каждая allocation имеет `0 < portion_factor <= 1`; сумма долей одной `plan_recipe` не превышает 1. Нераспределённая доля — явный резерв, а не потерянная еда. Meal и выделенная ему партия принадлежат одному плану. Fixed meal не имеет component allocations.

### 3.2 Nutrition quality rules (MVP ruleset)

Soft-правила поверх макро-целей (`docs/deep-research-report.md`). Для percentage-of-kcal фактическая доля считается от planned kcal оцениваемого дня: `sat_fat_pct = sat_fat_g × 9 / day_kcal`, `added_sugar_pct = added_sugars_g × 4 / day_kcal`. Диапазон kcal goal — hard feasibility, а не знаменатель quality-доли. Нулевой или неизвестный знаменатель даёт `unknown`. Принцип: **unknown ≠ 0** — неизвестный полезный нутриент не даёт credit, неизвестный рисковый даёт uncertainty-penalty.

| Код правила | Цель | Тип | Окно | Источник |
|---|---|---|---|---|
| `fiber_min_g` | ≥25 г/день (sex-specific: 25 ж / 35 м) | soft (reward) | день | WHO; NNR 2023 |
| `sodium_max_daily_mg` | <2300 мг/день | soft + warn | день | DGA; NNR |
| `added_sugar_max_pct_kcal` | <10% ккал (stretch ≤5% — warn) | soft | день | DGA; WHO |
| `sat_fat_max_pct_kcal` | <10% ккал | soft + warn | день | WHO; DGA; NNR |
| `industrial_trans_fat` | избегать; PHO или `industrial_trans_fat>0` → reject (HC-9); общий `trans_fat>0` → warn | hard если явно промышленные, иначе warn | — | WHO; NNR |
| `fruit_veg_min_g` | ≥400 г/день (stretch 500–800; без картофеля/соков) | soft (reward) | день | WHO; NNR |
| `whole_grain_ratio` | ≥50% злаков цельные (analytics: ≥90 г/день) | soft | день | DGA; NNR |
| `legumes_min` | ≥1 бобовый компонент/3-дн. план или ≥3 порции/нед | soft (reward) | план / 7d | WHO; NNR |
| `red_meat_max_weekly_g` | ≤350 г готового/нед, rolling-7d | soft + warn | 7d | NNR |
| `processed_meat` | в генерации цель = 0; user fixed разрешён, но warn | warn + soft penalty | 7d | NNR |
| `upf_share` | минимизировать, если класс известен; без hard-cutoff | analytics + soft penalty | день | WHO; NNR |

Food-group объёмы считаются в ожидаемом готовом эквиваленте:

```text
prepared_food_group_g = planned_input_qty_g
                      × prepared_qty_factor
                      × portion_factor
```

`prepared_qty_factor` принадлежит слоту рецепта, потому что выход зависит от способа готовки. Для ready-to-eat ингредиента он равен 1. Если фактор неизвестен, соответствующий food-group вклад не считается нулём: правило получает низкий coverage или `unknown`.

- `red_meat_g` и `processed_meat_g` — prepared-equivalent граммы соответствующих ингредиентов, суммируемые по eater в rolling-7d.
- `whole_grain_ratio` = whole-grain prepared equivalent / all-grain prepared equivalent. Неизвестная classification или yield снижает coverage знаменателя.
- `legume_servings` = legume prepared-equivalent grams / serving size из версионируемого ruleset. Размер порции не зашит в агрегатор.
- Coverage считается отдельно для каждого правила по его релевантным вкладам. Общий product coverage — справочная метрика и не может создать ложный `pass`.

#### Evaluation contract

Hard violations и soft evaluation — разные результаты. Явный PHO,
`industrial_trans_fat_per_100g > 0`, аллерген или hard-исключение отбрасывают
candidate и возвращаются как solver diagnostic. Они не кодируются значением
`soft_fail` и не участвуют в `quality_score`.

Границы MVP ruleset определены так:

| Правило | `pass` | `warn` | `soft_fail` / hard violation | Особый случай |
|---|---|---|---|---|
| `fiber_min_g` | значение `>=` sex-specific target | — | `< target` | неизвестная клетчатка → `unknown` |
| `sodium_max_daily_mg` | `< 2300 mg` | — | `>= 2300 mg` | соль переводится в натрий до агрегации |
| `added_sugar_max_pct_kcal` | `<= 5%` kcal | `> 5%` и `< 10%` | `>= 10%` | kcal `<= 0` или неизвестный numerator → `unknown` |
| `sat_fat_max_pct_kcal` | `< 10%` kcal | — | `>= 10%` | kcal `<= 0` или неизвестный numerator → `unknown` |
| `industrial_trans_fat` | PHO известен как false и industrial amount известен как 0 | общий `trans_fat > 0` при отсутствии industrial violation | PHO=true или industrial amount `> 0` — hard violation | неполные industrial поля без affirmative violation → `unknown` |
| `fruit_veg_min_g` | `>= 400 g/day` | — | `< 400 g/day` | картофель и соки не входят |
| `whole_grain_ratio` | ratio `>= 0.5` | — | ratio `< 0.5` | если all-grain denominator равен 0, ratio считается 1; объём `>=90 g/day` остаётся отдельной analytics-метрикой |
| `legumes_min` | `>=1` distinct allocated legume component/plan или `>=3` servings/rolling-7d | — | ниже соответствующей границы | один `plan_recipe` считается один раз на eater/plan, даже если распределён на несколько meals |
| `red_meat_max_weekly_g` | `<= 350 g/rolling-7d` | — | `> 350 g` | prepared-equivalent grams |
| `processed_meat` | 0 g | fixed-meal contribution `>0` | generated contribution `>0` | fixed meal не блокируется |
| `upf_share` | known share = 0 | known share `>0` | — | thresholdless continuous penalty `1-share`; unknown classification → `unknown` |

Per-rule coverage всегда находится в `0..1` и хранится вместе с evaluation:

- nutrient rules взвешивают известные значения по allocated input grams; salt и
  sodium считаются одним известным источником после конверсии;
- percentage-of-kcal rules используют известный planned kcal как denominator, а
  coverage относится к numerator;
- food-group rules требуют одновременно известные nullable classification flags
  и `prepared_qty_factor`; известный `false` отличается от NULL;
- rolling coverage агрегирует те же rule-specific known/total weights на всём
  окне; aggregate coverage — взвешенное среднее per-rule coverage, а не product
  `nutrition_coverage_score`;
- если coverage недостаточен для boundary decision, status=`unknown`; неизвестное
  никогда не даёт `pass`.

`quality_score` — informational число `0..100`, не замена лексикографической
целевой. Ruleset задаёт веса и нормализованные функции. Для пороговых soft-правил
базовые вклады: `pass=1`, `warn=0.5`, `soft_fail=0`, `unknown=0`; analytics-only
правила исключены, `upf_share` даёт `1-share`. Score eater/day — взвешенное
среднее; score плана — минимум по eater/day. Mean eater/day score используется
solver только следующим fairness tie-break. Aggregate coverage публикуется
отдельно и не может компенсировать плохой score.

Не делаем в MVP (см. `deep-research-report.md`): полный микронутриентный оптимизатор, UPF hard-ban, per-meal caps как стандарт, meal-timing/fasting как дефолт солвера, опору на новейшие спорные U.S.-сдвиги (база — пересечение WHO + EFSA/NNR + стабильная часть DGA).

### 3.3 Мягкие правила ротации (§7 DESIGN)

Вес/вероятность: ротация, анти-приедание, feedback, novelty, **soft-исключения едока** (HC-10 при `severity = soft`).

---

## 4. Ключевые концепции

**Рецепт = шаблон одной независимо дозируемой партии со слотами** (DESIGN §3). Целая тарелка не является `RecipeTemplate`: generated meal собирается из нескольких партий. Однородное рагу или карри может быть `mixed_main`, но его состав внутри порции всегда масштабируется пропорционально.

`RecipeTemplate` явно хранит:

- `component_kind = protein|carb_side|vegetable_side|sauce|mixed_main` — роль всей партии;
- `allowed_meal_types` — в каких приёмах её можно использовать;
- `base_servings > 0` — число стандартных порций базовой партии;
- optional `expected_output_g` — ручную оценку готового выхода;
- слоты с `qty_g` и optional `prepared_qty_factor`.

Слот ловит класс по тегу (`protein:lean:breast`), заполнение — конкретный продукт из личной БД. БЖУ слота = функция от заполнения, пересчитывается на лету при свопе.

**Подклассы ролей обязательны** (§12 Q1): грудка и бёдра отличаются по БЖУ и стоимости. `role_tag.code` = `kind:value[:subclass]`, иерархия через `parent_id`.

**Две библиотеки:** продукты (бренды с реальными БЖУ) и рецепты (растут от использования). Рецепт от LLM кладётся как `draft`; после первой реальной готовки + одобрения → `permanent`. Постоянные переиспользуются без LLM. Бан-листов нет — нежелательное просто не повышается (DESIGN §4).

**Партия и персональная порция — разные объекты.** `PlanRecipe` масштабирует базовый шаблон в общую партию. `MealComponentAllocation` связывает эту партию с конкретным meal через канонический `portion_factor`. Planned граммы готовой порции вычисляются только при известном expected output. БЖУ всегда распределяются по доле партии, поэтому усушка не меняет planned nutrition.

**План хранит снимок входа.** Для каждого eater/day сохраняются `generated_meal_types` и `macro_goal_id`, которые solver реально использовал. Более поздняя правка профиля не меняет старый план.

**Solver vs LLM** (DESIGN §5): ~70% детерминированно (Go/SQL), ~30% мягко. LLM в MVP активна по сути в одной точке — генерация рецептов.

**Макро-цели ≠ правила качества.** Личные макро-цели (`eater_macro_goal`) — что хочет пользователь; правила качества (3.2) — что делает рацион здоровым. Хранятся и считаются раздельно, не сливаются в один магический score.

**Coverage / uncertainty — отдельный слой.** Каждая quality-метрика знает, на сколько реальных данных опирается (`nutrition_data_quality`, coverage_score). Unknown клетчатка ≠ 0; unknown натрий ≠ безопасный; unknown processing ≠ minimally processed. Солвер оптимизирует не только качество, но и coverage — иначе продукты с плохими, но неизвестными значениями выглядят «чистыми» (структурный exploit).

**Nutrition summaries — снимки плана, а не глобальный кэш даты.** Day summary идентифицируется `plan + eater + date + basis`, rolling summary — `plan + eater + as_of_date + window + basis`. Результат правила уникален по `entity + eater + rule + basis`. Повторная evaluation той же версии плана заменяет эти строки в одной транзакции; другой план на те же даты их не перезаписывает.

---

## 5. Функциональные требования (user stories + acceptance criteria)

### FR-1 — Управление продуктами

> Как едок, я веду личную БД продуктов с реальными БЖУ, чтобы планы считались по тому, что я реально покупаю.

- CRUD продукта: бренд, название, БЖУ/100 г, ккал/100 г, цена/100 г, единица, аллергены, роли.
- **Optional quality-поля** (NULL = неизвестно): насыщенные жиры, транс-жиры/PHO, клетчатка, натрий и/или соль, сахара (всего/добавленные/свободные), food-группа, processing level, whole-grain ratio, флаги `is_legume`/`is_whole_grain`/`is_red_meat`/`is_processed_meat`, `counts_to_fruit_veg`. Известный `false`/`other` не используется как замена NULL.
- **Метаданные качества данных:** `nutrition_data_quality` (label_exact/trusted_db/estimated/unknown), `source_kind`, coverage_score, `last_verified_at`.
- AC: продукт без хотя бы одной роли нельзя использовать в слоте (валидатор предупреждает при создании рецепта/плана).
- AC: указание аллергенов продукта обязательно учитывается в HC-1/HC-2.
- AC: соль и натрий взаимоконвертируются (`натрий_мг = соль_г / 2.54 × 1000`); правила считают натрий.
- AC: незаполненные quality-поля не считаются нулями — они помечают продукт как low-coverage (влияет на ранжирование, FR-3).

### FR-2 — Управление профилем едока

> Как едок, я задаю цели БЖУ (диапазонами), аллергены и фиксированные приёмы, чтобы план строился под меня, и вижу историю изменения целей.

- CRUD профиля; CRUD аллергенов; **CRUD исключений по предпочтениям** (`eater_exclusion`); CRUD фиксированных приёмов; `sex` (для дефолта клетчатки).
- Цели БЖУ — диапазоны min–max для Б, Ж, У, ккал. Хранятся версионно (`eater_macro_goal` с `effective_from`); цель на дату = запись с максимальным `effective_from <= date`.
- **Quality-профиль отдельно от макро-целей:** `eater_quality_profile` (какой набор правил действует, `guideline_set_code`; в MVP read-only/default). Per-eater `eater_rule_override` (вкл/выкл правила, своя суровость/порог) — **v2/internal, в публичном API MVP не выставляется**; правила задаются в конфиге. Макро-цели остаются отдельным versioned-объектом.
- AC: правка цели создаёт НОВУЮ запись, не перезаписывает старую → история сохраняется и доступна (FR-8).
- AC: смена цели влияет на следующую генерацию, не на уже сгенерированные планы. План может пересечь `effective_from`; тогда каждый день использует версию, действующую на свою дату.
- AC: `sex` влияет только на дефолт клетчатки (25 ж / 35 м); `unspecified` → общий дефолт 25 г.
- AC: исключение задаётся по `food_group` / `role_tag` / `product` с `severity = hard|soft`; категория-уровень (food_group) покрывает группу продуктов одной записью (HC-10).
- AC: `fixed_meal` учитывается в дневном БЖУ и в quality-учёте (флаги `include_in_analytics`/`include_in_warnings`), но не генерируется и не свопается.

### FR-3 — Генерация плана (reuse-first)

> Как едок, я прошу план на 3 дня и получаю набор компонентов под мои БЖУ и constraints.

- Вход: `start_date`, положительное число дней (по умолч. 3), optional `cook_date`
  (по умолч. равен `start_date`), `novelty_level` 0–3 и непустой список уникальных
  per-eater targets (`eater_id` + явные `generated_meal_types`). Расписание target
  разворачивается в отдельный persisted `plan_eater_day` на каждую дату вместе с
  действующим `macro_goal_id`; одинаковое расписание не делает едоков одним профилем.
- Логика (§12 Q2 — **переиспользование**):
  1. Солвер ищет матчи среди `permanent`-рецептов под constraints.
  2. LLM вызывается **только** если permanent-кандидаты не дали допустимый план
     или `novelty_level = 3` явно просит новый шаблон. Уровень 2 ищет давно не
     использовавшийся компонент в существующей библиотеке.
  3. Сгенерированный рецепт проходит валидатор → кладётся `draft`.
- Солвер собирает 5–7 независимо дозируемых партий. Обычный набор содержит `protein`, `carb_side`, `vegetable_side` и optional `sauce`; `mixed_main` может заменить часть набора.
- Для каждого набора solver выполняет bounded deterministic search по сетке serving multipliers и отсекает ветки по hard constraints и достижимым min/max БЖУ. Сетка и search budget живут в конфиге solver, а не в каждом рецепте.
- Порции каждого eater/day рассчитываются отдельно в рамках общего набора компонентов. Совместность кандидата проверяется для конкретного едока: партия может остаться в общем плане для других людей.
- После поиска персональные serving units агрегируются в `plan_recipe.scale_factor`, planned slot-fill inputs и `meal_component_allocation.portion_factor`. Обычная генерация минимизирует резерв; после quantity adjustment или округления он может быть ненулевым.
- **Целевая функция — лексикографическая**: (1) hard safety/feasibility для всех → (2) macro-feasibility для всех → (3) сильнейшие quality-правила, сначала по худшему едоку, затем по среднему → (4) rolling-7d pattern, также worst-first → (5) cost / reuse / storage / novelty → (6) coverage.
- AC: каждый день у каждого едока сумма planned-БЖУ (включая `fixed_meal`) попадает в целевые диапазоны min–max из активной на эту дату `eater_macro_goal` (HC-8).
- AC: generated target не дублирует активный `fixed_meal` того же eater/meal type; клиент должен исправить явный target.
- AC: каждый generated meal имеет одну или несколько component allocations, а его planned macros/quality восстанавливаются из партий и долей без догадки о размере порции.
- AC: ни один компонент не нарушает HC-1…HC-10 (включая личные hard-исключения едока).
- AC: ответ генерации содержит **quality-оценку** плана: посчитанные правила (3.2), статусы `pass/soft_fail/warn/unknown`, агрегатный `quality_score`, список warnings с объяснениями.
- AC: **macro-pass, но quality-fail → план НЕ блокируется**, а понижается в ранжировании + явные warnings (решение владельца: ranking + warnings, не silent accept).
- AC: неизвестные данные снижают coverage/уверенность оценки, а не трактуются как «хорошо».
- AC: недельные правила оцениваются по rolling-7d горизонту (прошлый лог + текущий план), не только внутри 3-дневной сессии.
- AC: `novelty_level` 2 обязательно тащит permanent-компонент, не использованный 14+ дней; уровень 3 разрешает LLM сгенерировать новый draft candidate, который Go валидирует до повторного запуска solver (DESIGN §7).
- AC: novelty маппится в temperature LLM (0–0.2 → ~0.8) и пишется в `llm_call`.

### FR-4 — Свопы с blast radius

> Как едок, я меняю компонент и заранее вижу, во что это обойдётся.

- Перед применением свопа API возвращает предпросмотр радиуса (DESIGN §6):

  | Уровень | Что меняется | Последствия |
  |---|---|---|
  | fill→fill (рыба→рыба) | заполнение слота | ~0, БЖУ почти идентичны |
  | белок→другой белок | заполнение слота | пересчёт дня, возможно сдвиг порций гарнира |
  | блюдо→другое блюдо | весь шаблон | может сломать prep schedule |
  | вся ротация | план целиком | новая неделя + новый список покупок |

- AC: предпросмотр содержит diff БЖУ дня, **diff качества** (изменение клетчатки/натрия/sat fat/добавл. сахара, food-group объёмов, недельной экспозиции мяса, coverage), список затронутых приёмов, и — через `prep_dependency` — список блюд, которые станут невалидны.
- AC: своп применяется только после явного подтверждения; список покупок помечается как требующий перегенерации, если состав продуктов изменился.

### FR-5 — Список покупок

> Как едок, я получаю список покупок из плана.

- Производная от плана: агрегирование `slot_fill` по продукту, суммы количеств, оценка стоимости из `price_per_100g`.
- AC: один продукт — одна строка с суммарным количеством; чек-боксы `checked`.
- AC: перегенерация после свопа не теряет отметки по неизменившимся позициям.

### FR-6 — Логирование факта еды (planned vs actual)

> Как едок, я отмечаю руками, если съел не то/не так, чтобы трек БЖУ не был фикцией.

- По умолчанию приём = planned. Пользователь пишет actual только при отклонении (§12 Q3).
- Учитывается `cooked_by` / `logged_by` (§12 Q4 — household-учёт не привязан к числу едоков).
- AC: `actual_*` NULL ⇒ в аналитике берётся planned; иначе actual.
- AC: статусы `planned / eaten_as_planned / eaten_modified / skipped`.
- AC: **в MVP вручную вводятся только actual-МАКРОСЫ.** Actual quality-нутриенты руками не вводятся (чтобы не раздувать ввод). При `eaten_modified` без structured replacement quality-метрики приёма становятся `unknown`, а не подставляются planned — иначе quality-аналитика после факта была бы фикцией.
- AC: агрегаты `day_nutrition_summary` / `rolling_nutrition_summary` хранят `basis = planned|actual`; для actual quality считается только из известных structured-источников, неизвестное идёт как `unknown` (снижает coverage, не как 0).

### FR-7 — Feedback (вес, не бан)

> Как едок, я оцениваю/комментирую блюдо, чтобы влиять на вероятность его появления.

- `feedback_event` меняет `recipe_template.weight` (или вес продукта в выборе), не банит (DESIGN §7).
- AC: рейтинг −2..+2 / комментарий сдвигают `weight`; вес 0 не выпадает, но рецепт не удаляется.
- AC: в v1 комментарий хранится как текст; LLM-парсинг feedback — v2 (DESIGN §5 п.4).

### FR-8 — История и аналитика

> Как едок, через пару месяцев хочу «топ рецептов» и «средний белок по неделям».

- AC: эндпоинты агрегации по едоку/периоду: средние БЖУ (planned/actual), топ-N рецептов по `cooked_count`, частота продуктов.
- AC: **quality-аналитика**: средние клетчатка / натрий / sat fat / добавленный сахар; объёмы овощей-фруктов и цельных злаков; rolling-экспозиция красного/переработанного мяса; **data coverage по нутриентам**; **quality-нагрузка фиксированных приёмов** (вклад Сашиного завтрака).
- Схема заложена заранее (§12 Q5): `meal` индексирован по `(eater_id, meal_date)`; агрегаты — в `day_nutrition_summary` / `rolling_nutrition_summary`; результаты правил — в `rule_evaluation_result`.

### FR-9 — Повышение рецепта draft → permanent

> Как едок, после реальной готовки и одобрения я повышаю черновик до постоянного.

- AC: повышение ставит `status=permanent`, `promoted_at`, инкрементит `cooked_count`.
- AC: только `permanent` участвуют в reuse-first без LLM (FR-3).

---

## 6. Failure modes (graceful fail везде, §12 Q6)

Нигде не молчим — везде понятная ошибка.

- **LLM вернул продукт не из БД** → валидатор отбраковывает рецепт, ответ помечается невалидным, причина логируется в `llm_call.error`; клиент получает 422 с деталями.
- **LLM/таймаут/429/529** → retry с экспонентой (DESIGN §11), при исчерпании — 503 с пометкой «генерация недоступна, попробуйте reuse-only или позже».
- **Нет feasible-плана на заданной сетке** после полного bounded search → 422 `macro_infeasible` с per-eater/day диагностикой: минимум/максимум, достижимая граница, исключённые кандидаты и причины.
- **Исчерпан search budget до окончания перебора** → 422 `search_limit_reached` с `exhaustive=false`. Это не выдаётся за доказанную невозможность.
- **Своп ломает prep-граф** → не применяется молча; возвращается список невалидных блюд (FR-4).
- **Кэш-промах** — норм, считается заново; кэш-хит логируется как `status=cached`.
- **Quality-fail при macro-pass** → не молчание и не блок: план возвращается с `quality_score`, статусами правил и warnings; в ранжировании понижается. Правила с низким coverage возвращают статус `unknown`, а не ложный `pass`.

---

## 7. Нефункциональные требования

- Стоимость LLM несущественна (~$1/мес, DESIGN §11), но все вызовы логируются и кэшируются по хешу промпта.
- Промпт = код: версионируется, хранится в репе в файлах, тестируется на фиксированных примерах (DESIGN §11).
- Детерминизм через temperature; structured output через tool use / function calling → `json.Unmarshal` → Go-валидатор.
- Solver стабильно сортирует кандидаты и использует явный tie-break; одинаковый вход и конфиг дают одинаковый план.
- Логировать ВСЁ в `llm_call`: timestamp, prompt, response, latency, tokens_in/out, model, temperature.

---

## 8. Solver vs LLM (граница реализации)

**Детерминированно (Go, без LLM):** планировщик-солвер (лексикографическая целевая, FR-3), пересчёт БЖУ при свопе, валидатор рецептов, weighted random ротации/«удиви меня», список покупок, blast-radius и prep-граф, **rule evaluation + rolling-недельные счётчики + uncertainty handling + quality scoring + объяснения правил**, аналитика.

Определения quality-правил (3.2) живут в **конфиге/коде** (прагматичный MVP, без data-driven guideline-движка): версионируются и тестируются как промпты. В БД хранятся привязка набора к едоку, override'ы, агрегаты и результаты оценки.

**LLM (единственная активная точка в MVP):** генерация новых рецептов под constraints (режим «удиви меня» и пополнение библиотеки). Остальные три точки (голос, vision, парсинг feedback) — v2.

---

## 9. Стек

- **Go** — основной язык.
- **Postgres** — хранилище (продукты, рецепты, планы, факты, feedback, LLM-логи).
- **sqlc** — type-safe queries из SQL (`docs/schema.sql`).
- **OpenAPI-first** (oapi-codegen) — спека `api/openapi/api/api.yaml` → хендлеры/DTO.
- **Docker** — контейнеризация: `Dockerfile` для приложения (multi-stage Go build) + `docker-compose.yml` с Postgres для локальной разработки и запуска. Миграции прогоняются в составе compose. Owner-only деплой — один compose-стек.
- **LLM:** Claude Sonnet для MVP (строгий JSON, длинные системные промпты). Доступ через `internal/llm/client.go` с интерфейсом `LLMClient.Generate(ctx, prompt, schema)` + кэш/retry/timeout 30s (DESIGN §11).

Schema-first + sqlc + OpenAPI задают жёсткие границы для LLM-вайбкодинга: минимум места галлюцинировать сигнатуры.

**Auth (owner-only):** в v1 достаточно статического API-токена в заголовке (или доступ только из локальной сети compose). Полноценная авторизация — не в скоупе.

---

## 10. Соответствие открытым вопросам DESIGN §12

| # | Вопрос | Решение в ТЗ |
|---|---|---|
| Q1 | Гранулярность тегов | Подклассы: `role_tag.code` = `kind:value:subclass`, иерархия `parent_id` |
| Q2 | Регенерация vs переиспользование | Reuse-first (FR-3): permanent сначала, LLM при infeasible permanent-only результате или `novelty_level = 3` |
| Q3 | Подтверждение факта еды | Пользователь пишет actual руками при отклонении (FR-6) |
| Q4 | Кто готовит/логирует | Учёт `cooked_by`/`logged_by` в `meal` (FR-6) |
| Q5 | История и аналитика | Нужна; заложена в схему + FR-8 |
| Q6 | Failure modes | Graceful fail везде (§6) |
| Q7 | Usage-based weights | Не делаем — полей нет |
| Q8 | MVP-граница UI | HTTP API; бот — отдельный проект |

---

## 11. Решения по quality-слою (из deep-research-report.md)

| Вопрос | Решение |
|---|---|
| Объём quality-слоя в MVP | **Прагматичный MVP**: hot-path нутриенты + food-классификация + soft-правила + coverage-флаги + rolling-7d. Без generic nutrient table и без data-driven guideline-движка (правила — конфиг). |
| Per-eater override правил | **v2/internal** (`eater_rule_override` есть в схеме, но не в публичном API). В MVP правила — общий конфиг + `guideline_set_code` read-only. |
| Транс-жиры | hard reject только для промышленных (`PHO` или `industrial_trans_fat>0`); общий `trans_fat` → warn. |
| Actual quality при логировании | вручную не вводится; `eaten_modified` без structured replacement → quality `unknown`. |
| Целостность чисел | non-negative CHECK в БД + `minimum: 0` в API; `nutrition_coverage_score` — readOnly (считает сервер). |
| Macro-pass, quality-fail | **Ranking + warnings** (не silent accept, не hard block). |
| Фиксированные приёмы | **Учитываются в quality-аналитике и warnings** (ветчина = переработанное мясо), но не генерируются. |
| `sex` в профиле | **Да**, только для sex-specific дефолта клетчатки (25 ж / 35 м). |
| Rolling-7d | **Сразу в MVP** для недельных правил (мясо, бобовые). |
| Микронутриенты | Не в MVP (ложная точность при ручном вводе); generic-таблица — v2. |
| UPF | Аналитика + soft penalty, **не hard-ban**. |
| База правил | Пересечение WHO + EFSA/NNR + стабильная часть DGA; новейшие спорные U.S.-сдвиги не закладываем. |

Полное обоснование и источники — `docs/deep-research-report.md`.
