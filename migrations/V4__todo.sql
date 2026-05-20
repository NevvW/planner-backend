CREATE TABLE IF NOT EXISTS todo_tasks (
  id uuid NOT NULL,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title text NOT NULL,
  description text NOT NULL DEFAULT '',
  start_ms bigint NULL,
  deadline_ms bigint NULL,
  status text NOT NULL,
  priority text NULL,
  tags text[] NOT NULL DEFAULT '{}',
  type text NULL,
  task_done_time_ms bigint NULL,
  created_at_ms bigint NOT NULL,
  updated_at_ms bigint NOT NULL,
  deleted_at_ms bigint NULL,
  version bigint NOT NULL,
  PRIMARY KEY (id, user_id)
);

CREATE TABLE IF NOT EXISTS todo_sync_events (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  entity_id uuid NOT NULL,
  created_at_ms bigint NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_todo_tasks_user_updated_id
  ON todo_tasks(user_id, updated_at_ms DESC, id ASC);

WITH seed_users (id, email, login, password_hash) AS (
  VALUES
    (
      '11111111-1111-4111-8111-111111111111'::uuid,
      'anna.planner@example.com'::citext,
      'anna_planner',
      convert_to('$2a$12$pCU4ibt2zOSGuXQfzMp4tO7nrZPLjPAtdfLYl5WPUw8hnYOhyd0Hi', 'UTF8')
    ),
    (
      '22222222-2222-4222-8222-222222222222'::uuid,
      'max.manager@example.com'::citext,
      'max_manager',
      convert_to('$2a$12$pCU4ibt2zOSGuXQfzMp4tO7nrZPLjPAtdfLYl5WPUw8hnYOhyd0Hi', 'UTF8')
    ),
    (
      '33333333-3333-4333-8333-333333333333'::uuid,
      'sofia.design@example.com'::citext,
      'sofia_design',
      convert_to('$2a$12$pCU4ibt2zOSGuXQfzMp4tO7nrZPLjPAtdfLYl5WPUw8hnYOhyd0Hi', 'UTF8')
    )
)
INSERT INTO users (id, email, login, password_hash)
SELECT id, email, login, password_hash
FROM seed_users
ON CONFLICT (id) DO UPDATE SET
  email = EXCLUDED.email,
  login = EXCLUDED.login,
  password_hash = EXCLUDED.password_hash;

INSERT INTO user_profiles (user_id, profile_name, description, avatar_url, is_public)
VALUES
  (
    '11111111-1111-4111-8111-111111111111',
    'Анна Петрова',
    'Планирует учебу, работу и личные задачи.',
    NULL,
    true
  ),
  (
    '22222222-2222-4222-8222-222222222222',
    'Максим Орлов',
    'Ведет рабочие задачи команды и контроль дедлайнов.',
    NULL,
    true
  ),
  (
    '33333333-3333-4333-8333-333333333333',
    'София Смирнова',
    'Собирает дизайн-задачи и подготовку к релизам.',
    NULL,
    false
  )
ON CONFLICT (user_id) DO UPDATE SET
  profile_name = EXCLUDED.profile_name,
  description = EXCLUDED.description,
  avatar_url = EXCLUDED.avatar_url,
  is_public = EXCLUDED.is_public,
  updated_at = now();

WITH clock AS (
  SELECT (extract(epoch FROM now()) * 1000)::bigint AS now_ms
),
seed_tasks (
  id,
  user_id,
  title,
  description,
  start_offset_ms,
  deadline_offset_ms,
  status,
  priority,
  tags,
  type,
  done_offset_ms,
  created_offset_ms,
  updated_offset_ms
) AS (
  VALUES
    (
      'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa1'::uuid,
      '11111111-1111-4111-8111-111111111111'::uuid,
      'Закрыть отчет по бюджету',
      'Сверить расходы за неделю, приложить таблицу и отправить финальную версию.',
      -259200000::bigint,
      -86400000::bigint,
      'done',
      'Важно',
      ARRAY['работа', 'финансы'],
      'Задача',
      -90000000::bigint,
      -345600000::bigint,
      -90000000::bigint
    ),
    (
      'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa2'::uuid,
      '11111111-1111-4111-8111-111111111111'::uuid,
      'Позвонить поставщику',
      'Уточнить новые сроки доставки и записать итог в карточку проекта.',
      -172800000::bigint,
      -3600000::bigint,
      'active',
      'Важно',
      ARRAY['работа', 'звонок'],
      'Задача',
      NULL::bigint,
      -259200000::bigint,
      -7200000::bigint
    ),
    (
      'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa3'::uuid,
      '11111111-1111-4111-8111-111111111111'::uuid,
      'Подготовить план спринта',
      'Разложить задачи по приоритетам и вынести спорные пункты на встречу.',
      3600000::bigint,
      172800000::bigint,
      'active',
      'Важно',
      ARRAY['планирование', 'спринт'],
      'Задача',
      NULL::bigint,
      -86400000::bigint,
      -3600000::bigint
    ),
    (
      'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa4'::uuid,
      '11111111-1111-4111-8111-111111111111'::uuid,
      'Собрать идеи для отпуска',
      'Сохранить места, билеты и примерный бюджет без жесткого дедлайна.',
      NULL::bigint,
      NULL::bigint,
      'active',
      'Не важно',
      ARRAY['личное', 'путешествие'],
      'Задача',
      NULL::bigint,
      -604800000::bigint,
      -86400000::bigint
    ),
    (
      'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbb1'::uuid,
      '22222222-2222-4222-8222-222222222222'::uuid,
      'Провести ревью релизного чек-листа',
      'Проверить пункты деплоя, владельцев и rollback-план.',
      -345600000::bigint,
      -172800000::bigint,
      'done',
      'Важно',
      ARRAY['релиз', 'команда'],
      'Задача',
      -180000000::bigint,
      -432000000::bigint,
      -180000000::bigint
    ),
    (
      'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbb2'::uuid,
      '22222222-2222-4222-8222-222222222222'::uuid,
      'Ответить на вопросы QA',
      'Разобрать блокеры из тестового прогона и назначить ответственных.',
      -86400000::bigint,
      -1800000::bigint,
      'active',
      'Важно',
      ARRAY['qa', 'релиз'],
      'Задача',
      NULL::bigint,
      -172800000::bigint,
      -1800000::bigint
    ),
    (
      'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbb3'::uuid,
      '22222222-2222-4222-8222-222222222222'::uuid,
      'Запланировать 1:1 с командой',
      'Подготовить темы и временные слоты для коротких встреч.',
      86400000::bigint,
      259200000::bigint,
      'active',
      'Не важно',
      ARRAY['команда', 'менеджмент'],
      'Задача',
      NULL::bigint,
      -36000000::bigint,
      -1800000::bigint
    ),
    (
      'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbb4'::uuid,
      '22222222-2222-4222-8222-222222222222'::uuid,
      'Обновить заметки по процессам',
      'Дополнить внутреннюю памятку примерами частых решений.',
      NULL::bigint,
      NULL::bigint,
      'active',
      'Не важно',
      ARRAY['документация', 'процессы'],
      'Задача',
      NULL::bigint,
      -518400000::bigint,
      -72000000::bigint
    ),
    (
      'cccccccc-cccc-4ccc-8ccc-ccccccccccc1'::uuid,
      '33333333-3333-4333-8333-333333333333'::uuid,
      'Согласовать макеты онбординга',
      'Проверить финальные экраны, состояния ошибок и тексты кнопок.',
      -432000000::bigint,
      -259200000::bigint,
      'done',
      'Важно',
      ARRAY['дизайн', 'онбординг'],
      'Задача',
      -250000000::bigint,
      -604800000::bigint,
      -250000000::bigint
    ),
    (
      'cccccccc-cccc-4ccc-8ccc-ccccccccccc2'::uuid,
      '33333333-3333-4333-8333-333333333333'::uuid,
      'Передать иконки в разработку',
      'Экспортировать SVG, проверить названия слоев и приложить ссылку на Figma.',
      -172800000::bigint,
      -7200000::bigint,
      'active',
      'Важно',
      ARRAY['дизайн', 'assets'],
      'Задача',
      NULL::bigint,
      -259200000::bigint,
      -7200000::bigint
    ),
    (
      'cccccccc-cccc-4ccc-8ccc-ccccccccccc3'::uuid,
      '33333333-3333-4333-8333-333333333333'::uuid,
      'Собрать варианты пустых состояний',
      'Подготовить три варианта для задач без дедлайна и пустого списка.',
      7200000::bigint,
      345600000::bigint,
      'active',
      'Не важно',
      ARRAY['ui', 'исследование'],
      'Задача',
      NULL::bigint,
      -43200000::bigint,
      -3600000::bigint
    ),
    (
      'cccccccc-cccc-4ccc-8ccc-ccccccccccc4'::uuid,
      '33333333-3333-4333-8333-333333333333'::uuid,
      'Разобрать референсы для календаря',
      'Собрать удачные паттерны выбора даты и времени без ограничения по сроку.',
      NULL::bigint,
      NULL::bigint,
      'active',
      'Не важно',
      ARRAY['research', 'calendar'],
      'Задача',
      NULL::bigint,
      -691200000::bigint,
      -172800000::bigint
    )
)
INSERT INTO todo_tasks (
  id,
  user_id,
  title,
  description,
  start_ms,
  deadline_ms,
  status,
  priority,
  tags,
  type,
  task_done_time_ms,
  created_at_ms,
  updated_at_ms,
  deleted_at_ms,
  version
)
SELECT
  seed_tasks.id,
  seed_tasks.user_id,
  seed_tasks.title,
  seed_tasks.description,
  CASE
    WHEN seed_tasks.start_offset_ms IS NULL THEN NULL
    ELSE clock.now_ms + seed_tasks.start_offset_ms
  END,
  CASE
    WHEN seed_tasks.deadline_offset_ms IS NULL THEN NULL
    ELSE clock.now_ms + seed_tasks.deadline_offset_ms
  END,
  seed_tasks.status,
  seed_tasks.priority,
  seed_tasks.tags,
  seed_tasks.type,
  CASE
    WHEN seed_tasks.done_offset_ms IS NULL THEN NULL
    ELSE clock.now_ms + seed_tasks.done_offset_ms
  END,
  clock.now_ms + seed_tasks.created_offset_ms,
  clock.now_ms + seed_tasks.updated_offset_ms,
  NULL,
  1
FROM seed_tasks
CROSS JOIN clock
ON CONFLICT (id, user_id) DO NOTHING;
