# Доказательный аудит nutrition-модели для owner-only meal planner

## Executive summary

Текущий дизайн делает почти всю «правильность питания» функцией четырёх чисел — белки, жиры, углеводы, калории. Это видно и в формулировке задачи, и в MVP-скоупе, и в acceptance criteria генерации: система планирует на 2–3 дня под цели по БЖУ, а успех плана определяется попаданием planned-БЖУ в диапазоны по Б/Ж/У/ккал и соблюдением технических hard constraints. В сущностях и аналитике также первыми классами выступают в основном БЖУ/ккал и planned-vs-actual по макросам. fileciteturn0file0L10-L19 fileciteturn0file0L38-L40 fileciteturn0file0L100-L107 fileciteturn0file0L245-L255 fileciteturn0file1L11-L20 fileciteturn0file1L80-L90 fileciteturn0file1L98-L107 fileciteturn0file1L137-L155

С научной точки зрения это недостаточно. Крупные официальные руководства сходятся не только на калориях и макросах, но и на пищевом паттерне: основу должны составлять nutrient-dense и преимущественно minimally processed/unprocessed foods; важны ограничения по натрию, насыщенным жирам, добавленным или свободным сахарам; значимы клетчатка, фрукты/овощи, цельные злаки, бобовые; в долгосрочном паттерне важны ограничения красного и особенно переработанного мяса. WHO в 2026 прямо описывает здоровый рацион через адекватность, баланс, умеренность и разнообразие и называет фундаментом рациона разнообразные минимально обработанные продукты с низким содержанием вредных жиров, свободных сахаров и натрия. DGA в своей официальной executive summary также делает акцент на dietary pattern, nutrient-dense foods, овощи, фрукты, цельные злаки, бобовые и ограничение added sugars, saturated fat и sodium. NNR 2023 даёт конкретные европейски релевантные ориентиры по клетчатке, натрию, цельным злакам, овощам/фруктам и красному мясу. citeturn3view0turn9view0turn8view0turn12view0turn12view1turn13view0turn14view0turn13view2

Главный продуктовый риск у вас не в том, что система «ошибётся на 5 г углеводов», а в том, что она сможет собрать **macro-correct junk food**: нужный белок и калории можно набрать из солёных полуфабрикатов, переработанного мяса, сладких белковых десертов, рафинированного хлеба и соусов, потому что модель почти не видит клетчатку, натрий, насыщённые жиры, добавленный сахар, долю овощей, цельнозерновость и переработанность. Это не баг солвера; это пробел доменной модели. fileciteturn0file0L245-L255 fileciteturn0file1L98-L107 citeturn3view0turn8view0turn12view5

Отдельно надо прямо назвать встроенную в продукт неподдержанную нормализацию: фиксированный завтрак из хлеба, ветчины и сыра сейчас трактуется как нейтральный элемент, который надо просто учесть в БЖУ. С точки зрения evidence-based quality это неверно: ветчина — переработанное мясо; такие продукты NNR рекомендует держать «as low as possible», а DGA/WHO-совместимый паттерн в целом ограничивает продукты с избытком натрия и насыщенного жира. Это не значит «запретить пользователю ветчину»; это значит **перестать кодировать такой завтрак как безусловно нормальный baseline** и перевести его в слой user override с предупреждениями по quality. fileciteturn0file0L23-L26 fileciteturn0file1L37-L40 citeturn8view0turn9view0turn13view2

Итоговый вывод: для MVP вам не нужен «полноценный клинический нутриционист в коде». Нужен гораздо более практичный второй слой поверх пользовательских макро-целей: **evidence-based nutrition quality layer** с сильными soft constraints, rolling weekly metrics, предупреждениями, учётом неполных данных и явным разделением между «персональные макро-цели» и «общепопуляционные правила качества рациона». citeturn3view0turn8view0turn9view1turn10view0

## Evidence-backed nutrition principles for this product

### Что должно означать «питаться правильно» в вашем продукте

Для такого meal-prep продукта «есть правильно» должно означать не «попасть в макросы любой ценой», а «попасть в макросы **внутри** здорового пищевого паттерна». У WHO это формулируется как адекватность, баланс, умеренность и разнообразие, а у DGA — как healthy dietary pattern, built around nutrient-dense foods and beverages, с ограничением added sugars, saturated fat и sodium. NNR 2023 дополняет это конкретными food-based ориентирами для региона Европы и Скандинавии: клетчатка, цельные злаки, овощи/фрукты, бобовые, низкое потребление красного мяса и минимизация переработанного мяса. citeturn3view0turn8view0turn12view0turn12view1turn13view0turn13view1turn13view2turn14view0

Из этого следуют **две разные сущности**, которые в продукте нельзя смешивать. Первая — это personal macro goals: сколько белка, жира, углеводов и калорий пользователь хочет получать. Вторая — evidence-based quality rules: ограничения и предпочтения, которые делают рацион не только «сходящимся по числам», но и похожим на здоровый рацион по качеству. Если их смешать в одну таблицу или один score, вы получите либо магические веса без объяснимости, либо продукт, который маскирует плохую пищевую структуру под «норму», потому что пользовательские макросы выполнены. fileciteturn0file1L86-L105 citeturn8view0turn9view1turn10view0

### Какие правила реально поддержаны консенсусом

Сильный консенсус и хорошая переносимость в продукт есть у следующих вещей: ограничение натрия до менее 2,300 мг/сутки; ограничение насыщённых жиров до менее 10% энергии; ограничение добавленных или свободных сахаров до менее 10% энергии; avoidance/минимизация trans fat; достаточное потребление клетчатки; высокий вклад овощей и фруктов; приоритет цельных злаков; регулярное место бобовых в рационе; фокус на overall dietary pattern, а не на isolated nutrients. По всем этим пунктам есть либо межгосударственное согласие нескольких руководств, либо сильная опора на систематические обзоры, уже встроенные в NNR 2023 и DGA. citeturn9view0turn8view0turn12view0turn12view1turn13view0turn13view1turn14view0turn14view1

Для meal prep особенно важно, что WHO прямо допускает frozen и canned fruits/vegetables как хорошие варианты, если в них нет added sugars или excess sodium. Это очень хорошо ложится на ваш storage-first сценарий: quality layer не должен насильно тянуть в план «неживущие» заправленные салаты; нужный объём растений и клетчатки можно добирать через замороженные овощи, тушёные овощи, запечённые овощи, супы, рагу, бобовые и цельнозерновые гарниры. То есть доказательная nutrition quality **совместима** с 2–3-дневным хранением, если модель это умеет представлять. citeturn3view0turn13view0

### Какие пункты лучше считать спорными или контекстными

Ultra-processed foods — хороший пример того, что в продуктовой модели полезно, но в hard rules опасно. WHO в broad healthy-diet messaging тяготеет к minimally processed foundation, а NNR 2023 признаёт, что высокое потребление UPF ассоциировано с ожирением, CVD, T2D, раком, депрессией и mortality; при этом комитет NNR **не стал** формулировать отдельную количественную рекомендацию по UPF, потому что классификация NOVA не добавляет достаточно надёжного правила поверх уже существующих food-based рекомендаций. Дополнительно NNR подчёркивает, что некоторые whole grain breads, yoghurts, fish/fruit/vegetable products тоже могут попадать в UPF-классы в зависимости от формулировки. Для продукта это означает: UPF — полезная аналитика и penalty, но плохой hard gate для MVP. citeturn13view3

Meal timing — ещё более слабый кандидат на дефолтные правила. Важные general-population guideline documents, на которые логично опираться в вашем продукте, не задают для здоровых взрослых универсальные количественные правила по времени приёмов пищи; они концентрируются на dietary pattern и nutrient/food-group targets. Отсюда практический вывод: meal timing можно оставить user preference или analytics-only, но не превращать в default hard/soft optimizer objective в MVP. Это вывод-инференс из того, **что** именно нормируют эти руководства, а не отдельная клиническая рекомендация. citeturn3view0turn8view0turn10view0

Наконец, в 2026 американская guideline-среда стала менее стабильной: новейшая американская политико-регуляторная риторика сместилась, и это вызвало заметную критику за конфликт с предыдущим корпусом доказательств и за спорное сообщение по мясу/жиру. Для продукта, который должен быть устойчивым и practically correct, я бы не строил default-логику на самых новых и спорных U.S.-specific сдвигах. Для базового ruleset разумнее опираться на пересечение WHO + EFSA/NNR + longstanding DGA consensus, а недавние американские новации считать debated until stabilized. citeturn32news0turn34news1turn9view1turn9view2turn8view0

## Recommended MVP nutrition rule set

Ниже — не «идеальная диета», а **MVP ruleset**, который реально можно кодировать в meal-planning продукте с ручным вводом и неполными label data.

| rule | threshold/target | unit | hard/soft/warn/analytics-only | source | confidence | schema implication | solver implication |
|---|---:|---|---|---|---|---|---|
| Клетчатка | default ≥25; если есть sex-specific profile, можно default: 25 для женщин / 35 для мужчин | g/day | **soft** | WHO healthy diet: ≥25 g/day; NNR 2023: 25/35 g/day adults citeturn9view0turn12view0 | **High, consensus** | Нужны `fiber_g_per_100g`, coverage flags, daily aggregates | Сильный reward; неизвестная клетчатка не даёт credit |
| Натрий | <2300 | mg/day | **soft + warn when exceeded** | DGA executive summary; NNR sodium chronic disease risk reduction 2.3 g/day citeturn8view0turn12view1 | **High, consensus** | Нужны `sodium_mg_per_100g` и/или `salt_g_per_100g` + conversion | Сильный penalty за превышение; ещё penalty за low coverage |
| Added / free sugar | <10% энергии; stretch-goal ≤5% — только warning/analytics | % kcal, резолвится в g/day | **soft**, stretch = **warn** | DGA: added sugar <10% kcal; WHO: free sugar <10%, ≤5% may improve health citeturn8view0turn9view0 | **High for <10%; Moderate for 5%** | Нужны отдельные поля `added_sugars_g_per_100g`, `free_sugars_g_per_100g`, `total_sugars_g_per_100g` | Сильный penalty; если known only total sugar, не считать это прямым compliance |
| Насыщённые жиры | <10% энергии | % kcal, резолвится в g/day | **soft + warn when exceeded** | WHO; DGA; NNR fatty acids <10 E% citeturn9view0turn8view0turn14view1 | **High, consensus** | Нужен `saturated_fat_g_per_100g` | Сильный penalty; особенно важен для cheese/processed meat defaults |
| Промышленные trans fats | избегать; если явно есть PHO / industrial trans fat >0 → reject generated product | boolean / g/day | **hard if explicit**, иначе **warn/unknown** | WHO: trans fat ≤1% energy and industrial trans fats should be avoided; NNR: as low as possible citeturn9view0turn14view1 | **High, consensus** | Нужны `trans_fat_g_per_100g`, `contains_partially_hydrogenated_oils`, provenance | Generated plans не должны предлагать такие продукты, если флаг известен |
| Овощи и фрукты | base target ≥400; stretch 500–800 | g/day average across plan | **soft** | WHO ≥400 g/day; NNR 500–800 g/day, exclude potatoes/pulses and limit added/free sugar products citeturn3view0turn13view0 | **High, consensus** | Нужны food-group flags и aggregate grams; отдельный счётчик без картофеля/соков | Reward за растениеёмкость; легко совместимо с frozen/canned veg |
| Цельные злаки | primary MVP rule: ≥50% grain components whole-grain; analytics target when calculable: ≥90 g/day | ratio / g/day | **soft** | DGA: at least half grains whole grain; NNR: at least 90 g/day whole grains citeturn8view0turn14view0 | **High, consensus** | Нужны `whole_grain_ratio` и/или `whole_grain_g_per_100g_equiv` | Prefer whole-grain swaps over refined grains when macros permit |
| Бобовые | минимум 1 legume-based component на 3-дневный план **или** rolling target ≥3 servings/week | plan / week | **soft** | WHO includes pulses among preferred carb sources; NNR recommends pulses as significant regular part of diet citeturn3view0turn13view1 | **Moderate, consensus on direction but not one numeric global standard** | Нужны `is_legume` и rolling counters | Reward за inclusion; отличный storage-friendly источник fiber/protein |
| Красное мясо | ≤350 cooked g/week, rolling 7d | g/week | **soft + warn** | NNR red meat ≤350 g/week cooked; lower is better environmentally citeturn13view2 | **Moderate-to-high, consensus** | Нужны `red_meat_g_cooked_equiv` и rolling weekly aggregates | Penalize plans that project weekly overshoot |
| Переработанное мясо | generated meals target = 0; user fixed meals allowed but warned; long-run target “as low as possible” | weekly amount/frequency | **warn + soft penalty** | NNR: processed red meat as low as possible; it is also a sodium source citeturn13view2turn12view1 | **High on direction; no universal numeric cap** | Нужны `is_processed_meat`, `processed_meat_g` | Do not let solver rely on ham/sausages as normal protein solution |
| UPF | minimize if classification known; no default hard cutoff | share of kcal / item share | **analytics-only + soft penalty** | WHO favours minimally processed foundation; NNR notes associations but declines specific recommendation on UPFs citeturn3view0turn13view3 | **Moderate, debated / classification-sensitive** | Нужны `processing_level`, confidence, source | Penalty yes; infeasibility no |

Практическое правило для резолвинга процентных лимитов в user-specific grams:  
`grams_cap = target_kcal * percent / kcal_per_g`.  
Для ваших текущих диапазонов это даёт, например, saturated fat caps примерно **21.1–22.2 г/сутки** для Паши и **18.6–19.7 г/сутки** для Саши; added sugar <10% — примерно **47.5–50 г/сутки** и **41.8–44.3 г/сутки** соответственно. Эти числа должны **вычисляться автоматически из текущей kcal-цели**, а не храниться вручную как независимые константы. Пороговые проценты поддержаны WHO/DGA; сами граммы — прямое арифметическое следствие. fileciteturn0file1L35-L40 citeturn8view0turn9view0

На уровне типологии правил я рекомендую такой принцип: **hard only when product really knows the truth and user would expect deterministic enforcement**. Для здоровых взрослых без disease-state это означает: hard — аллергены, явные запреты пользователя, storage/equipment, макро-цели, явный industrial trans fat flag. Всё остальное из nutrition quality — прежде всего soft constraints, warnings и analytics, потому что label coverage в реальном grocery-данном мире неполна. Это особенно важно для России/Восточной Европы и ручного MVP-ввода. fileciteturn0file1L46-L58 citeturn9view1turn13view3

## What not to implement in MVP and why

Не стоит делать **полный микронутриентный оптимизатор**. Для general healthy adults без клинических состояний и при ручном вводе продуктов это даст много ложной точности и мало реальной пользы: по большинству брендовых labels у вас просто не будет надёжных данных по калию, кальцию, йоду, витамину D, железу и т.д. Хранить extensible nutrient model полезно уже сейчас, но превращать микронутриенты в solver constraints в MVP — нет. Максимум — optional analytics и долгосрочные sentinel warnings, если позже появится доверенный food-composition layer. citeturn9view1turn10view0

Не стоит делать **UPF hard ban**. Сама evidence-base по неблагоприятным ассоциациям серьёзная, но category boundary остаётся спорной, а NNR 2023 сознательно отказался от отдельной normative recommendation по UPF. Кроме того, некоторые продукты, которые продуктово удобны и nutritionally acceptable, могут получать UPF-классификацию по формуле/ингредиентам. В MVP UPF должен быть аналитикой и penalty, а не главным gatekeeper’ом. citeturn13view3

Не стоит вводить **универсальные per-meal caps** по натрию, сахару или насыщённым жирам как доказательный стандарт. Официальные general-population guides в основном задают **daily** или **weekly/pattern** limits. Если вы хотите UX-эвристику вроде «один meal не должен нести >60% дневного sodium budget», гэта можно сделать как предупреждение, но надо честно маркировать как product heuristic, а не как guideline rule. Это вывод из структуры самих руководств. citeturn8view0turn9view0turn10view0

Не стоит делать **meal timing / fasting windows дефолтной логикой солвера**. Для healthy adults это пока слишком context-dependent, а major guidelines, на которые вы разумно хотите опираться, не превращают время приёма пищи в базовый количественный target. Если пользователь захочет собственное eating window preference — это отдельный configurable layer, но не evidence-based default for MVP. citeturn3view0turn8view0turn10view0

Не стоит принимать недавние и спорные **новые американские политические сигналы** как базу продукта. Сейчас U.S. guidance environment менее стабилен, и recent changes уже вызывают критику за конфликт с прежним консенсусом. Для owner-only продукта лучше опереться на устойчивое пересечение WHO, EFSA/NNR и давно закреплённой части DGA, чем зашивать в код новейшие и contested сдвиги. citeturn32news0turn34news1turn8view0turn9view2

## Concrete schema.sql diff proposal in plain English

Текущая доменная модель полезна как каркас, но nutrition-часть надо разделить на **hot-path fields**, **classifications**, **rules**, **aggregates**, **provenance** и **user overrides**. Сейчас бизнес-логика по сути знает только макросы, аллергены, роли, storage/equipment и planned-vs-actual macros. fileciteturn0file0L245-L255 fileciteturn0file1L80-L90

### Product-level nutrient fields

В таблицу `product` не стоит просто «добавить ещё двадцать nullable-колонок и забыть». Лучше сделать гибрид:

**Добавить в `product` hot-path поля**, которые солвер реально будет часто использовать:
- `saturated_fat_g_per_100g`
- `trans_fat_g_per_100g`
- `fiber_g_per_100g`
- `sodium_mg_per_100g`
- `salt_g_per_100g`
- `total_sugars_g_per_100g`
- `added_sugars_g_per_100g`
- `free_sugars_g_per_100g`
- `nutrition_data_quality` enum: `label_exact | trusted_db | estimated | unknown`
- `nutrition_coverage_score` numeric
- `source_kind` enum: `pack_label | store_card | composition_table | manual_estimate`
- `source_note`
- `last_verified_at`

Почему и `salt_g`, и `sodium_mg`: потому что в потребительских данных часто встречается именно соль, а quality rules и guidelines почти всегда считают натрий. Нормализованный `effective_sodium_mg` должен считаться через conversion layer; NNR напоминает, что 1 г sodium ≈ 2.54 г salt. citeturn12view1

**Добавить generic nutrient table** для будущих micronutrients:
- `nutrient_def(id, code, label, unit, category, is_hot_path)`
- `product_nutrient_value(product_id, nutrient_id, amount_per_100g, value_status, source_id, confidence_score)`

Так вы не заблокируете v2+ на кальции, калии, железе, йоде, витамине D, но и не будете тащить всё это в горячий MVP join-path.

### Food classification fields and tables

Добавьте food-quality классификацию отдельно от nutrient fields. Макросы и quality — не одно и то же.

Минимально нужны:
- `product_food_classification`
  - `product_id`
  - `food_group` enum: `fruit | vegetable | potato | grain | dairy_alt | dairy | legume | nuts_seeds | fish_seafood | egg | red_meat | white_meat | processed_meat | mixed | sauce_condiment | confectionery | beverage | other`
  - `processing_level` enum: `minimally_processed | processed_culinary | processed_food | ultra_processed | unknown`
  - `processing_confidence`
  - `is_fruit`
  - `is_vegetable`
  - `counts_to_fruit_veg`
  - `is_legume`
  - `is_whole_grain`
  - `whole_grain_ratio`
  - `is_red_meat`
  - `is_processed_meat`
  - `contains_partially_hydrogenated_oils`
  - `classification_source`
  - `classification_note`

Отдельно стоит хранить **counting semantics**, а не пытаться выводить их из food group на лету. Например:
- картофель не должен идти в `fruit_veg_g`
- fruit juice не должен считаться как whole fruit
- legumes лучше считать отдельным каналом, даже если вы позже захотите показывать их и внутри plant-food analytics

Это особенно важно для food-based rules из WHO/NNR. citeturn3view0turn13view0turn13view1turn14view0

### Guideline and rule tables

Evidence-based rules не должны быть hardcoded в solver constants. Они должны жить в данных.

Добавьте:
- `guideline_set(id, code, label, population_scope, region_scope, version, is_default, active_from, active_to)`
- `guideline_source(id, org, title, publication_year, url, source_type, citation_text)`
- `guideline_set_source(guideline_set_id, source_id)`
- `nutrition_rule(id, guideline_set_id, code, label, subject_scope, metric_code, threshold_type, threshold_value, threshold_unit, comparator, severity_default, evidence_strength, consensus_state, note)`
- `nutrition_rule_threshold_resolver(rule_id, resolver_kind)`  
  Например: `percent_of_kcal`, `rolling_7d_sum`, `daily_absolute`, `grain_ratio`, `weekly_frequency`

Ключевая идея: хранить **абстрактное правило** и отдельно способ его резолвинга. Например:
- `sat_fat_max_pct_kcal = 10`
- `added_sugar_max_pct_kcal = 10`
- `sodium_max_daily_mg = 2300`
- `red_meat_max_rolling_7d_g = 350`

Тогда solver сначала превращает rule set в resolved constraints для конкретного eater/profile/plan window.

### Daily aggregate and result tables

Сейчас вам не хватает модели, которая способна сказать не только «попали ли мы в белок», но и «какой quality-профиль у дня/плана» и «насколько этим оценкам можно доверять».

Добавьте:
- `meal_nutrition_summary`
  - planned/actual hot-path nutrients
  - `fruit_veg_g`
  - `whole_grain_g_equiv`
  - `legume_g`
  - `red_meat_g`
  - `processed_meat_g`
  - `upf_kcal_share_est`
  - `data_coverage_score`
- `day_nutrition_summary`
  - по eater_id + date
  - все агрегаты дня
  - separate `planned_*` и `actual_*`
- `rolling_nutrition_summary`
  - по eater_id + as_of_date + `window_days`
  - для 3/7/14 дней
  - чтобы weekly rules можно было оценивать даже при 3-day planning horizon
- `rule_evaluation_result`
  - `entity_type` (`meal|day|rolling_window|plan`)
  - `entity_id`
  - `eater_id`
  - `rule_id`
  - `status` (`pass|soft_fail|warn|unknown`)
  - `actual_value`
  - `target_value`
  - `coverage_score`
  - `message`

Это даст explainability: пользователь увидит не только «план хороший», а почему именно он хороший или где компромисс.

### Evidence, provenance and incomplete-data tracking

Это критично для MVP.

Добавьте:
- `nutrition_value_source`
  - `id`
  - `source_kind`
  - `raw_label_text` nullable
  - `entered_by`
  - `entered_at`
  - `reference_name`
  - `region`
  - `confidence_score`
- `product_nutrient_completeness`
  - `product_id`
  - `has_sodium`
  - `has_sat_fat`
  - `has_fiber`
  - `has_added_sugar`
  - `has_free_sugar`
  - `has_trans_fat`
  - `has_processing_classification`

Смысл простой:
- **unknown ≠ 0**
- beneficial nutrients with unknown values should **not** count as achieved
- risk nutrients with unknown values should generate **uncertainty penalty / warning**
- solver must optimize not only nutrition quality, but also **coverage quality**

Иначе у вас появится структурный exploit: продукты с плохими, но неизвестными значениями будут выглядеть «чистыми».

### User-specific overrides

Отдельно от `eater_macro_goal` нужны overrides для quality layer:
- `eater_quality_profile(id, eater_id, guideline_set_id, active_from, note)`
- `eater_rule_override(id, eater_id, rule_id, enabled, severity_override, threshold_override, rationale, active_from)`
- `fixed_meal_quality_override(id, fixed_meal_id, ignore_for_solver, include_in_warnings, include_in_analytics)`

Это позволит честно моделировать реальность:
- макро-цели — личные
- quality defaults — evidence-based
- отклонения от defaults — осознанные user overrides
- fixed meals не ломают систему, но и не становятся «невидимыми»

## Concrete DESIGN.md and SPEC.md changes

### Что переписать в DESIGN.md

**Раздел «Задача».** В первой формулировке после слов про БЖУ нужно прямо добавить, что система оптимизирует не только Б/Ж/У/ккал, но и nutrition quality: клетчатку, sodium/sat fat/sugars limits, food-group balance и weekly pattern constraints. Иначе документ продолжит транслировать ложную архитектурную аксиому, что «не переедать» = «попасть в макросы». fileciteturn0file0L10-L19 citeturn3view0turn8view0

**Раздел «MVP».** Нужно расширить scope формулировкой вроде: `ручной ввод продуктов включает optional nutrition-quality fields и data-quality metadata; генерация плана учитывает evidence-based quality rules поверх personal macro goals; аналитика показывает macro compliance, quality compliance и coverage quality`. Сейчас MVP сформулирован слишком узко и закрепляет macro-only модель. fileciteturn0file0L38-L40

**Раздел «solver vs LLM».** В детерминированную часть обязано войти: rule evaluation, rolling weekly counters, uncertainty handling, quality scoring, rule explanations. Сейчас там есть planner, macro recalculation и validator, но нет слоя, который защищает от macro-correct junk food. fileciteturn0file0L100-L109

**Раздел «План на 2 сеанса в неделю».** Добавьте правило, что часть quality constraints must be evaluated not only per generated 3-day session but over a rolling 7-day horizon. Иначе red meat / processed meat / legumes у вас концептуально некуда поставить. fileciteturn0file0L161-L170 citeturn13view1turn13view2

**Раздел «Краткая модель данных».** Его надо переписать как минимум до следующего уровня:
- `Product` = brand fields + hot-path nutrients + data quality + classification
- `Plan/Meal` = macros **and** nutrition-quality aggregates
- `GuidelineSet / NutritionRule / RuleEvaluation`
- `NutritionValueSource`
- `RollingSummary`
Сейчас концептуальная модель недоописывает именно ту часть домена, которую вы хотите исследовать. fileciteturn0file0L241-L255

### Что переписать в SPEC.md

**Раздел «Доменные правила».** Его нужно развести на две группы:
- `hard feasibility & safety constraints`
- `nutrition quality rules`
Сейчас всё важное про питание за пределами БЖУ отсутствует, а hard constraints описывают в основном аллергию, storage, equipment и fixed meals. fileciteturn0file1L46-L58

**FR-1 Управление продуктами.** Нужно расширить CRUD продукта: кроме белка/жира/углеводов/ккал и аллергенов, продукт должен принимать optional fields по saturated fat, fiber, sodium/salt, sugars, food-group classification, processing level, whole-grain ratio и metadata качества данных. Иначе quality layer некуда записывать. fileciteturn0file1L78-L82

**FR-2 Профиль едока.** Нужен новый объект рядом с macro goals: `quality profile`. Там должны храниться guideline set, overrides, whether fixed meals affect warnings, optional sex-specific defaults for fibre, и т.д. Макро-цели должны остаться отдельным versioned объектом. fileciteturn0file1L86-L92

**FR-3 Генерация плана.** Acceptance criteria должны быть переписаны. Недостаточно `planned-БЖУ попадает в диапазоны`. Нужно минимум:
- plan satisfies hard safety/feasibility constraints
- plan returns nutrition quality evaluation
- solver maximizes quality score subject to macro feasibility
- unknown data reduce quality confidence
- weekly rules use rolling counters  
Именно здесь должна появиться защита от junk-food solutions. fileciteturn0file1L98-L107

**FR-6 Логирование факта еды.** planned vs actual надо расширить не только на macros, но и на hot-path quality nutrients/aggregates, по крайней мере на уровне derived summary. Иначе аналитика качества после фактического потребления всегда будет фикцией. fileciteturn0file1L137-L140

**FR-8 История и аналитика.** Эндпоинты должны показывать не только average macros and top recipes, но и:
- average fibre / sodium / sat fat / added sugar coverage
- fruit/veg and whole-grain averages
- red/processed meat rolling exposure
- data coverage by nutrient
- fixed-meal quality burden  
Иначе analytics останется макро-дэшбордом, а не nutrition-quality system. fileciteturn0file1L150-L155

### Что надо поменять в OpenAPI contract

В API нужны не косметические патчи, а новый слой DTO:

- `NutritionFacts`
  - macros + hot-path nutrients + completeness metadata
- `FoodClassification`
  - group, processing, whole-grain, legume, meat flags
- `NutritionQualityRuleEvaluation`
  - rule code, status, actual, target, coverage, message
- `NutritionQualitySummary`
  - day/plan summaries, rolling-week projections, score, warnings
- `QualityProfile`
  - guideline set + user overrides

Практически:
- `ProductInput` и `Product` должны перестать быть macro-only
- `FixedMealInput` должен уметь принимать full nutrition, иначе fixed breakfast останется невидимым для quality
- `Plan` response должен включать `quality_summary`, `rule_evaluations`, `coverage_summary`
- `/analytics/macros` нужно дополнить или заменить на более общий `/analytics/nutrition-quality`
- swap preview должен возвращать не только macro diff, но и quality diff

## Open questions for the product owner

- Готовы ли вы считать **фиксированные приёмы пищи** частью quality analytics и warnings, даже если они не участвуют в генерации? Если нет, система будет системно недооценивать quality burden fixed breakfast. fileciteturn0file0L23-L26
- Хотите ли вы хранить в профиле едока `sex` только для выбора default fibre target, или оставляете общий adult default 25 g/day без sex-specific logic? citeturn9view0turn12view0
- Готовы ли вы вручную проставлять хотя бы coarse food classifications (`is_legume`, `is_processed_meat`, `whole_grain_ratio`, `processing_level`) при вводе продукта? Без этого quality layer резко потеряет силу.
- Что должно происходить, если план попадает в макросы, но нарушает quality rules: silent accept, warning badge, or downgrade in ranking? Я рекомендую ranking + explicit warnings, не silent accept.
- Нужен ли rolling 7-day контекст сразу в MVP, или вы готовы временно оценивать weekly rules через “projected week” из прошлого лога + текущего плана?
- Приемлемо ли для owner-only продукта иметь несколько режимов генерации: `macro-first balanced-quality`, `quality-first within macro slack`, `reuse-only`? Это может резко улучшить explainability.
- Какой источник trusted estimates вы готовы принять для unlabeled staples вроде гречки, овсянки, сырой птицы, фасоли, замороженных овощей: curated internal table, национальная composition table, или пока только manual exact values?

## Final recommended architecture

Архитектурно я бы рекомендовал жёстко разделить систему на **четыре слоя**.

Первый слой — **personal macro goals**. Это то, что у вас уже есть и что должно остаться отдельно: versioned user-specific targets по белку, жирам, углеводам и калориям. Эти цели принадлежат пользователю, а не «науке». Они могут меняться, и продукт не должен делать вид, что они равны guideline truths. fileciteturn0file1L35-L42

Второй слой — **evidence-based nutrition quality rules**. Это default ruleset для generally healthy adults on weight loss, основанный на WHO + NNR/EFSA + стабильной части DGA consensus. Он содержит soft constraints, warnings и analytics, а не клинические назначения. Эти правила должны быть data-driven через `guideline_set` и `nutrition_rule`, а не зашиты в код как случайные коэффициенты. citeturn3view0turn8view0turn9view1turn9view2

Третий слой — **coverage and uncertainty layer**. Это то, чего чаще всего не хватает в diet apps. Каждая quality-метрика должна знать, насколько она основана на реальных данных. Unknown fibre ≠ zero fibre. Unknown sodium ≠ safe sodium. Unknown processing level ≠ minimally processed. Solver и analytics должны работать с `known / estimated / unknown` и penalize low coverage. Иначе система будет системно предпочитать продукты с неполными labels. Это особенно критично в вашей географии и при manual entry MVP. citeturn9view1turn13view3

Четвёртый слой — **hierarchical solver objective**. Я бы делал не один общий score, а лексикографическую оптимизацию:

1. hard safety / feasibility  
2. personal macro feasibility  
3. strongest quality rules  
4. rolling weekly pattern quality  
5. cost / reuse / storage convenience / novelty  
6. data completeness  

Именно так вы сохраните практичность. Система сначала находит реализуемый meal-prep plan для двух людей с вашими storage/equipment constraints, а потом среди feasible plans выбирает тот, который не просто «влезает в БЖУ», а ещё и имеет более здоровый food pattern. Это и есть правильный способ сохранить личные macro goals **отдельно** от evidence-based nutrition quality, не превращая одно в другое. fileciteturn0file0L25-L26 fileciteturn0file0L100-L107 fileciteturn0file1L98-L107 citeturn3view0turn8view0turn10view0

Если сформулировать в одной строке: **макросы должны оставаться пользовательским слоем планирования, а “eat correctly” в продукте должно означать additional quality rules on top of macros, with evidence tracking, rolling pattern metrics and uncertainty-aware optimization.** citeturn3view0turn8view0turn9view2