# AnalyseApp

![CI](https://github.com/whitegauss/analyseapp/actions/workflows/ci.yml/badge.svg)

実験データの解析（グラフ化・回帰分析など）を簡単に行うためのWebアプリ。設計の詳細は [PDR.md](./PDR.md) を参照。

## ドキュメント

| 読みたいもの | 場所 |
| --- | --- |
| なぜこれを作るのか、何を目指すのか | [PDR.md](./PDR.md) |
| 動かし方、機能の使い方、API の仕様 | この Readme |
| コードがどう分かれているか、なぜそう分かれているか | [docs/architecture.md](./docs/architecture.md) |
| テストの書き方とこの repo 固有の約束事 | [docs/testing.md](./docs/testing.md) |

**テストを触る前に [docs/testing.md の「現状固定テスト」](./docs/testing.md#現状固定テスト)を読むこと。** 既知のバグは「いまの壊れた挙動」をテストで固定してあるので、バグを直すとテストが赤くなる。それは成功のしるしであって、修正を戻す理由ではない。

## 技術スタック
- **API Gateway/BFF**: Go（[go-chi](https://github.com/go-chi/chi) + [zerolog](https://github.com/rs/zerolog)）
- **解析ワーカー**: Python（FastAPI + structlog + numpy、将来的にgRPC常駐プロセス化を予定）
- **キャッシュ**: Redis（[go-redis](https://github.com/redis/go-redis)）。解析結果を`analysis:{experiment_id}:{type}:{params_hash}`キーで24hキャッシュ（PDR.md §7）
- **DB/認証**: Supabase（PostgreSQL / Auth）。Go APIが `DATABASE_URL` で直接Postgresに接続する唯一の経路（[pgx](https://github.com/jackc/pgx)）。認証はSupabase AuthのJWT（JWKS/ES256）をGo APIが検証（[golang-jwt](https://github.com/golang-jwt/jwt) + [keyfunc](https://github.com/MicahParks/keyfunc)）
- **フロントエンド**: Next.js 16（App Router）/ React 19 / TypeScript 5 / Tailwind CSS v4 / ESLint 9（`@/` エイリアス構成）。認証は [@supabase/ssr](https://github.com/supabase/ssr) でCookieベースのセッション管理（Server Actions + `proxy.ts`でのセッションリフレッシュ）。グラフ描画は [Plotly.js](https://plotly.com/javascript/)（PDR.md §6）。軸ラベルの数式表示は [KaTeX](https://katex.org/)
- **コンテナ/開発基盤**: Docker Compose

現状はバックエンド（SupabaseのPostgres接続・スキーマ・JWT認証・experiments CRUD・Python Workerでの線形回帰解析）に加え、フロントエンドのログイン画面と実験データ入力・グラフ表示まで実装済みです。ログイン後のトップページ（`/`）はプロジェクトのダッシュボードで、プロジェクトを開いて（`/projects/{id}`）データ貼り付け→ライブプレビュー→保存という流れになります。保存後は`/experiments/{id}`で確認でき、`/experiments`ではプロジェクト横断で実験を一覧できます。トップページ下部には保存せずグラフだけ描く下書き欄があります（ログイン不要）。軸ラベルは1つのテキスト欄に`$v$`のように`$...$`で数式部分を囲んで入力すると、その部分だけKaTeXで数式（変数は斜体）として描画され、それ以外は単位・日本語も含めそのまま立体表示されます（PDR.md §6の`axis_label_runs`の考え方をベースにしたシンプル版）。`/experiments/{id}`では線形回帰の回帰直線がグラフに自動で重ね描画されます（オフにもできます）。

## 使用方法
1. Docker と Docker Compose が利用できる環境を用意します。
2. リポジトリ直下の `.env.example` を `.env` にコピーし、`DATABASE_URL`（Supabase Dashboard > Connect > Connection string）・`SUPABASE_URL`（Project Settings > API > Project URL）・`NEXT_PUBLIC_SUPABASE_URL`/`NEXT_PUBLIC_SUPABASE_ANON_KEY`（同Project URL / anon public key）などを設定します。
3. 初回のみDBマイグレーションを実行します（`profiles`/`api_keys`/`experiments`/`analysis_results`テーブルを作成）。
   ```bash
   docker compose --profile tools run --rm migrate
   ```
4. 下記の起動方法に従ってサービスを立ち上げます。

## 起動方法
### 全サービスをまとめて起動
```bash
docker compose up --build
```
- フロントエンド: `http://localhost:3000`（ログイン中はトップページがプロジェクトのダッシュボード、`/login`・`/signup`）
- Go API: `http://localhost:8080`（liveness: `GET /healthz`、readiness/DB疎通: `GET /readyz`）
- Python Worker: `http://localhost:8001`（ヘルスチェック: `GET /healthz`）
- Redis: `localhost:6379`

### 個別サービスの再起動例
- フロントエンドだけ: `docker compose restart frontend`
- APIだけ: `docker compose restart api`
- Workerだけ: `docker compose restart worker`

### 終了
```bash
docker compose down
```

## デプロイ（Vercel / Hobby プラン）

フロントエンドを Vercel に載せる前提の手順。**Vercel に載るのは Next.js だけ**で、Go API・Python Worker・Redis は別のホストが要る（下記「残りの3サービス」参照）。

### Vercel 側の設定

| 項目 | 値 |
| --- | --- |
| Root Directory | `frontend` |
| Framework Preset | Next.js（自動検出） |
| Build / Install | 既定のまま（`vercel.json` 以外に上書き不要） |

環境変数は3つ。**`NEXT_PUBLIC_*` はビルド時にバンドルへ埋め込まれる**ので、Production だけでなく Preview にも設定しておくこと。

| 変数 | 値 | 備考 |
| --- | --- | --- |
| `NEXT_PUBLIC_SUPABASE_URL` | Supabase の Project URL | ブラウザに露出する |
| `NEXT_PUBLIC_SUPABASE_ANON_KEY` | Supabase の anon public key | ブラウザに露出する |
| `API_INTERNAL_URL` | Go API の**公開 HTTPS URL** | ローカルと違い `http://api:8080` のような私設アドレスは使えない |

未設定のまま動かすと `lib/env.ts` が変数名を名指しで落とす。以前は `?? "http://localhost:8080"` に落ちていて、「Go API が落ちている」ように見えるだけで原因が分からなかった。

### 実行リージョン

`frontend/vercel.json` で `hnd1`（東京）を指定している。**Hobby は1リージョンのみ**で、既定は `iad1`（ワシントン D.C.）。

**合わせる先は利用者の居場所ではなく Supabase と Go API の場所。** 関数は1リクエストで Supabase Auth と Go API に何度も往復するので、そこが遠いと往復ぶんだけ遅くなる（静的ファイルは世界中の PoP から配られるのでこの設定とは無関係）。Supabase を東京以外に置いているなら `vercel.json` の値もそちらに変えること（[リージョン一覧](https://vercel.com/docs/regions#region-list)。大阪は `kix1`）。

### `output: "standalone"` を Vercel では切っている

`next.config.ts` が `process.env.VERCEL` を見て切り替える。standalone は Docker イメージ用（`frontend/Dockerfile` が `.next/standalone` を使う）で、Vercel は自前の Next.js アダプタを使うため不要。

**Next 16.3.0 では、これが付いたままだとビルドが落ちる。** Vercel の `onBuildComplete` が `.next/next-server.js.nft.json` を探すが、standalone モードではもう出力されない（[vercel/next.js#96646](https://github.com/vercel/next.js/issues/96646)）。**ローカルの `next build` は通る**ので手元では気付けない。

### 残りの3サービス

`docker-compose.yml` の4サービスのうち、Vercel に載るのは `frontend` だけ。

| サービス | 状況 |
| --- | --- |
| Go API | **要・別ホスト**。ブラウザからは直接呼ばれない（Server Component / Server Action 経由のみ）が、Vercel からは公開 HTTPS で届く必要がある。JWT 必須・レート制限・セキュリティヘッダーは実装済み |
| Python Worker | **要・別ホスト**。Go API からのみ呼ばれる |
| Redis | **任意**。解析結果のキャッシュ専用で、到達できなくてもキャッシュ無しで動作する（`lib/cache`）。最初は無しで出して構わない |
| Postgres | 不要。Supabase を使っているので既に外部 |

Vercel 自身にも Go・Python(FastAPI) の公式ランタイムと、`Dockerfile.vercel` による OCI イメージ実行があるので、3つとも Vercel に寄せる選択肢もある。どこに置くかは未決（KAN-32）。

### Hobby プランの制約

- **非商用のみ。** 業務・収益目的の利用は Pro 以上が要る
- 関数: メモリ 2GB / 1 vCPU、最大実行時間 300 秒、リクエスト/レスポンス本文 4.5MB
- Fast Data Transfer 100GB/月、1日あたり 100 デプロイ
- 関数は production で2週間、preview で48時間アクセスが無いとアーカイブされ、次の呼び出しでコールドスタートが1秒以上延びる

## テスト / Lint / CI

`main`へのpush・PRで [.github/workflows/ci.yml](./.github/workflows/ci.yml) がフロントエンド/Go API/Python Workerを並列でlint・フォーマットチェック・テスト・ビルドします。

### まとめて実行する

```bash
scripts/test.sh              # 3スタック全部。成功すると1スタック1行だけ出力する
scripts/test.sh api          # 単一スタック（all | frontend | api | worker）
scripts/test.sh --cov        # カバレッジ付き
scripts/test.sh --full       # 失敗時の出力を省略せず全部出す
```

**Go のリポジトリ層（SQL）のテストだけは実 DB が要ります。** `TEST_DATABASE_URL`を設定すると api スイートに含まれ、未設定なら飛ばして`(no db)`と表示します。使い捨ての Postgres を1つ立てるだけです。

```bash
docker run --rm -d -p 5433:5432 -e POSTGRES_PASSWORD=postgres --name analyseapp-test-db postgres:16
TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5433/postgres?sslmode=disable' scripts/test.sh api
docker rm -f analyseapp-test-db
```

`DATABASE_URL`ではなく専用の変数にしてあるのは、これらのテストが行を作っては消すためです。変数を1つ設定したままにしただけで本番 DB に向く、という事故の余地を残していません。

失敗したときは**失敗したスイートの出力だけ**を表示します（既定で末尾80行、`MAX_FAIL_LINES`で変更可）。全文は常に`.test-logs/<suite>.log`に残るので、切り詰められても失われません。いずれかのスイートが落ちると非ゼロで終了します。

Python Workerは`WORKER_PYTHON`、`backend/worker/.venv/bin/python`、`python3`の順にインタープリターを探します。ローカルでは`backend/worker/.venv`を作っておくのが楽です。

**現在のカバレッジ（2026-08-30 / `origin/main` @ 605cc65 時点）**

| スタック | カバレッジ | 測り方 |
| --- | --- | --- |
| Go API | 50.2% | `go test -coverpkg=./...`（テストを持たないパッケージも分母に含める） |
| フロントエンド | 22.01% | Vitest v8 / `coverage.include`（テストが読み込んだファイルだけでなく`app`・`components`・`lib`の全ファイルが分母） |
| Python Worker | 99% | `pytest --cov=app` |

閾値ゲートはまだ設定していません（KAN-36の作業中は数値が動き続けるため）。

### スタックごとに実行する

- フロントエンド: `cd frontend && npm install`
  - テスト: `npm run test`（[Vitest](https://vitest.dev/)。`lib/pasteDataParsing.ts`の貼り付けデータパース、`components/AxisLabel.tsx`の`$...$`区切りロジック、`components/ExperimentChart.tsx`の有効数字丸めロジックなど、純粋関数のみを対象。コンポーネントのレンダリングは未カバー — `environment: "node"`のため）
  - Lint: `npm run lint`（ESLint / `eslint-config-next`）
  - フォーマット: `npm run format`で整形、`npm run format:check`でCIと同じチェックのみ（[Prettier](https://prettier.io/)、`.prettierrc.json`）
  - ビルド: `npx next build`
- Go API: `cd backend/api`
  - テスト: `go build ./... && go vet ./... && go test ./...`（`internal/response`のエンベロープ整形、`internal/httpserver`のハンドラー検証ロジックをfake store（`experiments.Store`インターフェース）でテスト。DB は不要）
  - リポジトリ層のSQLテスト: `TEST_DATABASE_URL=... go test -tags=integration ./...`（`experiments.Repository`／`projects.Repository`の SQL を実 Postgres に当てて検証。**fake store では絶対に検知できない種類の退行** —— `and user_id = $2`が消える、`on delete cascade`が効かなくなる、`on conflict do nothing`の競合対策が壊れる —— を拾うためのもの。ビルドタグで分けてあるので、タグ無しの`go test ./...`は DB 無しで通ります。スキーマは`cmd/migrate`と同じ goose 経路で作るので、本番と同じ DDL に当たります）
  - Lint/フォーマット: [golangci-lint](https://golangci-lint.run/)（`.golangci.yml`）。`golangci-lint run ./...`でlint、`golangci-lint fmt ./...`でフォーマット（`--diff`で差分確認のみ）
- Python Worker: `cd backend/worker && pip install -r requirements-dev.txt`
  - テスト: `pytest`（`tests/test_linear_regression.py`が解析ロジック、`tests/test_main.py`がHTTP層・エラーエンベロープをカバー。設定は`pytest.ini`）
  - Lint/フォーマット: [Ruff](https://docs.astral.sh/ruff/)（`ruff.toml`）。`ruff check .`でlint、`ruff format .`でフォーマット（`--check`で差分確認のみ）

## API（experiments）

Go API GatewayのAPI構成は [backend/api/openapi.yaml](./backend/api/openapi.yaml)（OpenAPI 3.0 / Swagger）にまとめています。[Swagger Editor](https://editor.swagger.io/) 等に貼り付けると一覧・スキーマを確認できます。

**プロジェクト機能への移行中**（2026-08-16〜、ステージ分けと現在地は [PDR.md](./PDR.md) §5「プロジェクト機能への移行ステージ」を参照）: Stage 2 まで完了し、実験は必ず1つのプロジェクトに属します（`experiments.project_id`は`NOT NULL`）。プロジェクト側は`GET/POST/PATCH/DELETE /api/v1/projects`・`/api/v1/projects/{id}`に加えて、配下の実験を扱う以下の2つがあります。

- `GET /api/v1/projects/{id}/experiments` — そのプロジェクト配下の実験を作成日時の新しい順に一覧取得（他人のIDや存在しないIDは404。中身が空のプロジェクトは空配列を返す200で、404とは区別されます）
- `POST /api/v1/projects/{id}/experiments` — `{title, raw_data, config?}` を送信し、URLで指定したプロジェクトに実験を作成（他人のIDや存在しないIDは404）

フロントエンドの実験作成はこのネストしたパスを使います。下記のフラットな`POST /api/v1/experiments`もプロジェクトを指定しない作成パスとして残していますが、画面からは呼ばれません。

`/api/v1/experiments`系のエンドポイントは全てSupabase AuthのJWT（`Authorization: Bearer <access_token>`）が必須です。curlで試す場合、Supabase AuthのREST APIでサインアップ/サインインしてトークンを取得できます。

```bash
curl -X POST "$SUPABASE_URL/auth/v1/signup" \
  -H "apikey: $NEXT_PUBLIC_SUPABASE_ANON_KEY" -H "Content-Type: application/json" \
  -d '{"email":"you@example.com","password":"..."}'
# レスポンスの access_token を使う
```

- `POST /api/v1/experiments` — `{title, raw_data, config?}` を送信し実験を作成（`title`は省略・`null`可。空文字は`null`として保存されます）。プロジェクトを指定しない作成パスで、実験は既定プロジェクト「未分類」に入ります（無ければ自動作成）
- `GET /api/v1/experiments` — 自分が作成した実験をプロジェクト横断で作成日時の新しい順に一覧取得（ページネーションなし。コピー元の選択・複数実験の比較で使うため、プロジェクト配下の一覧とは別に残しています）
- `GET /api/v1/experiments/{id}` — 自分が作成した実験を取得（他人のIDや存在しないIDは404）
- `DELETE /api/v1/experiments/{id}` — 自分が作成した実験を削除（他人のIDや存在しないIDは404）。関連する`analysis_results`行はDBの`ON DELETE CASCADE`で一緒に削除されます
- `PATCH /api/v1/experiments/{id}/config` — `{config}` でグラフ設定を丸ごと置き換え
- `PATCH /api/v1/experiments/{id}/raw_data` — `{raw_data}` でデータ本体を丸ごと置き換え。成功するとこの実験のRedisキャッシュ済み解析結果（`analysis:{experiment_id}:*`）を全て無効化します（ベストエフォート。Redisに到達できない場合も更新自体は成功します）
- `PATCH /api/v1/experiments/{id}/project` — `{project_id}` を送信し、その実験の所属プロジェクトを付け替え（**実験IDは変わりません**。移動元の実験・移動先のプロジェクトのいずれかが他人のものや存在しないIDなら404。現在と同じプロジェクトを指定した場合も何も変わらず200）。キャッシュキーは`analysis:{experiment_id}:*`でプロジェクトを含まず`raw_data`も変わらないため、解析結果の無効化は発生しません
- `POST /api/v1/experiments/{id}/copy` — `{project_id}` を送信し、その実験を指定プロジェクトに複製（新しい実験IDが発番され、以後コピー元とは完全に独立します。コピー元の実験・コピー先のプロジェクトのいずれかが他人のものや存在しないIDなら404）
- `POST /api/v1/experiments/{id}/analyze` — `{type, params?}` を送信し、その実験の`raw_data`に対して解析を実行（例: `{"type":"linear_regression"}`）。`linear_regression`は`params`に`x_log`/`y_log`（真偽値、既定`false`）を渡すとlog10(x)・log10(y)に対して回帰します（片対数・両対数フィット。非正の値を持つデータ点はそのフィットから除外され、有効な点が2点未満なら`insufficient_data`エラー）。Go APIが実験を取得したうえでPython Workerの`POST /analyze`に中継し、Workerのレスポンス（`{data, error, meta}`）をそのまま返します。Workerに到達できない場合は`502`（`worker_unreachable`）。成功した結果はRedisに24hキャッシュされ（`analysis:{experiment_id}:{type}:{params_hash}`、PDR.md §7）、レスポンスヘッダー`X-Cache: HIT`/`MISS`でキャッシュ命中を確認できます。キャッシュ命中時はDB・Worker呼び出し自体が発生しません。Redisに到達できない場合もキャッシュなしで通常通り動作します

### セキュリティ・監視

- `/api/v1`配下は接続元IP単位で100リクエスト/分にレート制限しています。超過すると`429`（`rate_limited`）が返ります（接続元IPはTCP接続自体から解決しており、リクエストヘッダーは信用していません。将来リバースプロキシを前段に置く場合は解決方法の見直しが必要です）
- 全レスポンスに`X-Content-Type-Options`・`X-Frame-Options`・`Referrer-Policy`・`Strict-Transport-Security`のセキュリティヘッダーを付与しています（JSON APIのみのため`Content-Security-Policy`は設定していません）
- `GET /metrics`でPrometheus形式のメトリクス（`http_requests_total`・`http_request_duration_seconds`をメソッド・ルートパターン・ステータスコード別に、`http_panics_total`をメソッド・ルートパターン別に、`analysis_cache_results_total`を`/analyze`のキャッシュhit/miss別に）を公開しています。panicしたリクエストは`http_requests_total`にステータス500として記録されるため`status=~"5.."`のアラートで拾えます。`http_panics_total`はそれをコードが意図して返した500と区別するためのものです。**ただしレスポンスヘッダー送出後にpanicした場合は例外**で、送出済みのステータス（例:201）のまま記録されます（送出後はミドルウェアもRecovererもステータスを変えられないため）。この場合`status=~"5.."`のアラートには現れないので、`http_panics_total`が唯一の手がかりになります。なお`http.ErrAbortHandler`によるpanicは意図的な中断であり、`net/http`もRecovererも500を返さないためpanicとしては記録しません。`/healthz`/`/readyz`と同様に認証なし・スクレイピングするPrometheusサーバー自体は未構築（本番運用時は外部公開しないようネットワーク側で制限してください）

## ヘッダー・フッター（フロントエンド）

全ページ共通のヘッダー・フッターを`app/layout.tsx`に組み込んでいます（`components/Header.tsx`/`components/Footer.tsx`）。

- ヘッダー左側にハンバーガーメニュー（`components/ToolsMenu.tsx`）と「AnalyseApp」ロゴ（クリックでトップページへ）。ハンバーガーメニューは実験・ログイン状態と無関係に常時表示され、「計算ツール」から`/tools`へ遷移します。右側にログイン状態を表示: ログイン中なら「すべての実験」リンク（`/experiments`。ロゴがすでにダッシュボード＝プロジェクト一覧への導線なので、こちらは横断ビューにしています）・メールアドレス・ログアウトボタン、未ログインなら「ログインしていません」表示とログインボタン
- ヘッダーは全ページ（トップ・ログイン・新規登録・実験一覧・実験詳細）に共通で表示され、Supabaseの認証状態をサーバー側で毎リクエスト確認します（`/login`・`/signup`もこの影響で動的レンダリングになります）
- フッターはGitHubリポジトリへのリンクのみのシンプルな構成

## 計算ツール（フロントエンド、`/tools`）

実験データ（`/experiments/*`）とは独立した、ログイン不要のスタンドアロンなミニツールページです。ヘッダー左上のハンバーガーメニュー（`components/ToolsMenu.tsx`）の「計算ツール」から`/tools`に遷移します。タブ切り替え（`components/tools/ToolsCalculator.tsx`）で4つのツールを提供:

- **誤差伝播**（`ErrorPropagationCalculator.tsx`）: 数式を自由に入力すると（例: `2*pi*sqrt(L/g)`）、式に出てくる変数が入力欄として並び、各変数の値と1σ不確かさから z±σz を計算します。各変数は独立・無相関という前提での線形近似（1次のテイラー展開）による誤差伝播 σz = √Σ(∂z/∂xi・σxi)² を使用。偏微分は前進モードの自動微分で厳密に求めます（数値微分と違い刻み幅の選び方に依存しません）。変数ごとの ∂z/∂x と、分散に占める寄与率（どの測定を改善すると効くか）も表示します。式のパーサーと自動微分は`lib/formula.ts`、伝播の計算は`lib/errorPropagation.ts`
  - 使える関数: sqrt / exp / ln / log10 / sin / cos / tan / asin / acos / atan / atan2 / sinh / cosh / tanh / abs、定数`pi`・`e`。三角関数の引数はラジアン（度は`sin(theta*pi/180)`と書く）。`log`は底が曖昧なため意図的に受け付けず、`ln`か`log10`を促すエラーを返します。σを空欄にした変数は厳密な定数として扱われます
- **有効数字の丸め**（`SignificantFigureRounder.tsx`）: 値と1σ不確かさを入力すると、グラフの回帰直線表示と同じ規約（不確かさの先頭有効数字に合わせて丸め、小数点以下は最大4桁まで）で表示を計算します。丸めロジック自体は`lib/significantFigures.ts`（`roundToUncertainty`/`formatUncertainty`）としてグラフ側と共有しています
- **単位変換**（`UnitConverter.tsx`）: 長さ・質量・時間・角度・温度の5カテゴリ（`lib/unitConversion.ts`）。温度のみ単純な係数ではなくオフセット付きの変換式（℃/℉/K）を使用
- **統計量**（`MeasurementStatsCalculator.tsx`）: 測定値をスプレッドシートなどからコピー＆ペースト（複数列可）すると、列ごとにn・平均±不確かさ（平均の標準誤差）・標準偏差を計算します。保存済み実験とは無関係に、その場で貼り付けた数値だけを対象にする独立ツールです（`lib/statistics.ts`のロジックを再利用。以前は`/experiments/{id}`にインライン表示していましたが、実験に紐付かない汎用ツールとして`/tools`に統合しました）

## プロジェクト（フロントエンド）

ログイン後のトップページ（`/`）がプロジェクトのダッシュボードです。プロジェクト一覧の独立したルート（`/projects`）は作らず、`/`がそれを兼ねています。

- 各行にプロジェクト名と配下の実験件数。クリックでその場に実験一覧が開きます（`components/ProjectAccordion.tsx`）。展開は取得済みデータの表示切り替えだけで、追加のリクエストは発生しません
- ダッシュボードは`GET /api/v1/projects`と`GET /api/v1/experiments`を1回ずつ読み、`project_id`で突き合わせます（`lib/dashboard.ts`の`groupExperimentsByProject`、純粋関数なのでテストあり）。プロジェクトごとに`GET /api/v1/projects/{id}/experiments`を呼ぶとプロジェクト数だけリクエストが増えるうえ、件数表示のためにどのみち全実験が必要なためです
- 「+ 新しく作る」でプロジェクトを作成、展開した中の「名前を変更」で名前・説明を編集（どちらもインラインフォーム、`components/InlineEditCard.tsx`を実験側と共用）
- 「削除」はその場でインライン確認に切り替わります（ブラウザのalert/confirmは使いません）。**プロジェクトを削除すると配下の実験も`ON DELETE CASCADE`で一緒に消える**ため、確認文にその件数を出します
- 展開した中の「+ 実験を追加」から`/projects/{id}`へ。このページは配下の実験一覧と実験追加フォームで、保存は`POST /api/v1/projects/{id}/experiments`（ネストしたパス）を使います
- `/experiments/{id}`の「他のプロジェクトへコピー」からコピー先を選んで複製できます（`POST /api/v1/experiments/{id}/copy`）。コピー元はそのまま残り、コピーは新しい実験IDを持つ独立したレコードになります。コピー後はコピー先の`/experiments/{id}`へ遷移します。現在のプロジェクトも選択肢に残していて（「（現在のプロジェクト）」と表示）、同じプロジェクト内での複製もできます
- `/experiments/{id}`の「他のプロジェクトへ移動」から所属を付け替えられます（`PATCH /api/v1/experiments/{id}/project`）。コピーと違って複製されず、**実験IDが変わらない**ので既存のリンクやCSVダウンロードURLはそのまま使えます。移動先の選択肢には現在のプロジェクトは出ません（移動しても何も変わらないため）。プロジェクトが1つしかない場合は移動先が無いのでボタン自体が出ません

## 実験データ入力・グラフ表示（フロントエンド）

実験データの入力フォーム（`components/ExperimentEditor.tsx`）は2つのモードで使われます。

- **`/projects/{id}`の「実験を追加」**: 保存できるモード。保存先はURLのプロジェクトに固定されます（`project_id`は`NOT NULL`なので、保存先を決めずに保存する経路はありません）
- **トップページ下部の「グラフをすぐ見る」**: **保存しないモード**。貼り付けたデータをその場で描くだけで、DBには何も書きません（リロードすると消えます）。パースもグラフ描画も完全にクライアント側で完結するため、APIも認証も使わず**未ログインでも使えます**（`/tools`と同じ扱い）

- タイトルは任意（未入力なら`null`で保存され、表示時は「(無題)」になります）
- スプレッドシートからのコピー＆ペースト想定（タブ／カンマ／スペース区切りを自動判定）。1列目=`x`、2列目=`y`固定
- 3列目以降は列ごとに役割を選択（`y_error` / `x_error` / 使わない / カスタム名）。Python Workerの`DataSeries.columns`と同じキー名で保存されるため、将来の解析連携にそのまま使える
- **貼り付けた瞬間にクライアント側だけでPlotly.jsのグラフがライブ更新される**（保存前のプレビュー）。データが無い間・Plotly読み込み中はスケルトン（`components/ChartSkeleton.tsx`）を表示
- 軸ラベルの入力欄はデータ貼り付け前から常時表示。1つのテキスト欄にまとめて記入し、`$...$`で囲んだ部分だけTeXの数式としてKaTeXでレンダリング（変数は自然に斜体になる）、それ以外の部分（単位や日本語など）はそのままDOM表示（立体）にする方式（例: `速度 $v$ (m/s)`）。作成フォームで設定した内容がグラフのプレビューにもそのまま反映される（説明はラベルの横の「i」アイコンにカーソルを合わせると表示）
- グラフはPlotlyのデフォルトのx=0/y=0のゼロラインを非表示にしています
- 凡例（データ・回帰直線）を枠線付きでグラフ内（左上）に常に表示します。マウスでドラッグしてグラフ内の好きな位置に移動でき、グラフ下の「凡例の文字サイズ」スライダーで文字サイズ（8〜24px）を変更できます。凡例のテキストはグラフ上でダブルクリックすると直接編集できます（回帰直線側は初期状態で回帰式`y = (a ± σa)x + (b ± σb)`のように傾き・切片双方の1σ不確かさ付きで表示。データ側は「データ」のまま）。凡例内の`$...$`で囲んだ部分は軸ラベルと同じ`$...$`記法で斜体表示されます（Plotly純正の凡例のため、軸ラベルのKaTeX表示とは異なりPlotly自体のフォントでの近似斜体になります）
- 保存できるモードで「保存してグラフを確定」を押すとGo APIに保存され、`/experiments/{id}` にリダイレクトして確定版のグラフを表示（`y_error`/`x_error`があればエラーバー付き、軸ラベルも保存した内容で表示）
- `/projects/{id}`・`/experiments/{id}`は未ログインだと`/login`にリダイレクトされる（トップページの「グラフをすぐ見る」だけは未ログインでも使えます）
- `/experiments/{id}`表示時にサーバー側で`POST /api/v1/experiments/{id}/analyze`（`type: linear_regression`）を自動実行し、回帰直線をグラフの枠いっぱいに重ね描画します（データ点の範囲だけでなくプロット領域の端まで伸ばして表示）。回帰直線の周囲には傾き・切片それぞれの±1σ標準誤差を組み合わせた不確かさの帯（薄い赤の塗りつぶし）も表示します（共分散は考慮しない簡易的な envelope で、傾き・切片それぞれの±1σの4通りの組み合わせのうち両端で最も広がる線を採用。厳密な信頼区間ではありません）。「回帰直線を表示」チェックボックスでオン/オフ可能（既定はオン、帯も連動）。傾き・切片・R²も表示。傾き・切片はそれぞれの1σ標準誤差の最初の有効数字までで丸めて表示します（例: 傾き=2.04 ± 0.05、切片の誤差も同様に表示）。標準誤差が小さい場合は小数点以下、大きい場合（十の位・百の位など）は整数側の桁でも丸められます（例: 標準誤差52.3なら値517→520）。表示する小数桁数は最大4桁までに制限しています（不確かさが非常に小さい場合でも桁数が際限なく増えないように）。標準誤差が0（近似が完全すぎて桁落ちする場合など）は`0.0`と表示します。データ不足などで解析が失敗した場合は回帰直線なしで生データのみ表示されます（ページ全体は壊れません）
- 回帰直線は保存前のライブプレビュー・「グラフをすぐ見る」には表示されません（解析は保存済み実験の`raw_data`に対してのみ実行されるため）
- `/experiments`で保存済みの実験をプロジェクト横断で一覧できます（作成日時の新しい順、タイトル・所属プロジェクト名・作成日を表示、クリックで`/experiments/{id}`へ）。ヘッダーの「すべての実験」とダッシュボードの「すべての実験を見る」から遷移します。プロジェクト名は`GET /api/v1/projects`を併せて読んで`project_id`で突き合わせています（`lib/dashboard.ts`の`attachProjectTitles`）。1件も無い場合は空状態メッセージを表示
- 一覧の各行に「削除」ボタンがあり、押すとその場でインラインの確認（「削除する」/「キャンセル」）に切り替わります（ブラウザのalert/confirmダイアログは使用しません）。削除すると`/experiments`に戻り一覧から消えます
- `/experiments/{id}`の「軸ラベルを編集」ボタンから軸ラベル（X軸・Y軸）を事後編集できます（インラインフォーム、`PATCH /api/v1/experiments/{id}/config`を呼び出し）。保存すると同じページを再読み込みし、更新後の内容がグラフに反映されます
- `/experiments/{id}`の「データを編集」ボタンからも同様にデータ本体（`raw_data`）を事後編集できます。作成時と同じスプレッドシート貼り付け形式（テキストエリア＋3列目以降の役割選択）で、現在のデータを再構成した状態で編集を開始できます（`PATCH /api/v1/experiments/{id}/raw_data`を呼び出し、`raw_data`は丸ごと置き換え）。データを編集すると、Redisにキャッシュされていたこの実験の解析結果（回帰直線など）は自動的に無効化され、次回表示時に再計算されます
- `/experiments/{id}`の「CSVダウンロード」リンクから生データをCSVでダウンロードできます（`GET /experiments/{id}/export`、Next.jsのRoute Handler。Go APIへは同じくサーバー側でCookieのセッションを使ってアクセスするため未ログインなら`/login`にリダイレクト）。1行目以降は`#`始まりのコメント行でタイトル・作成日時・軸ラベル・回帰係数（回帰結果が取得できた場合、線形フィット固定）を記載し、続けてヘッダー行（軸ラベル設定済みならその文字列、無ければ`x`/`y`等の列名）とデータ本体を出力します
- 「X軸を対数表示」「Y軸を対数表示」チェックボックスで対数グラフに切り替えられます（保存前のプレビューでも表示のみ切り替え可能）。両方チェックすると両対数、片方だけなら片対数になります。保存済み実験（`/experiments/{id}`）でこれらをオンにすると、回帰直線もlog10変換後のデータに対して再フィットし直します（線形フィットをそのまま対数軸に重ねるのではなく、対数を取った後に改めて最小二乗フィットするため、両対数なら冪乗則・片対数なら指数関係がグラフ上で直線になります）。凡例・グラフ下の統計表示にも`log₁₀(x)`/`log₁₀(y)`を用いた式が表示されます。フィットは軸設定ごとに`/analyze`をクライアントから呼び出して取得し（初回のみ、以降は同一ページ内でキャッシュ）、非正の値を持つデータ点が多く残り2点未満になるなど計算できない場合はその旨のメッセージを表示します
- `/experiments`一覧の各行にチェックボックスがあり、2件以上選択すると「選択したN件を比較」リンクが有効になり`/experiments/compare?ids=id1,id2,...`へ遷移します（`components/ExperimentListWithCompare.tsx`）。比較ページ（`components/ComparisonChart.tsx`）は各実験の生データを実験ごとに色分けした散布図として1つのPlotlyグラフに重ね描画し、回帰直線（常に線形フィット固定、対数軸切り替えは無し）も同じ色の破線で各実験自身のデータ範囲内に重ねます。軸ラベルは最初に見つかった非空のものを使用（実験ごとに異なるラベルの統一表示は非対応）し、Plotly純正の軸タイトルには`$...$`のKaTeX記法は使えないため`$`記号は表示上取り除きます。存在しない/他人のID（404）はその実験だけ比較対象から除外され、有効な実験が2件未満になった場合は`/experiments`に戻ります

## Python Worker（解析）

`POST /analyze`（`http://localhost:8001/analyze`）に実験データを送ると数値解析結果を返します。認証やDB永続化は行いません（Go APIが将来仲介する想定、PDR.md §6）。

実験データは固定の`x`/`y`列ではなく**名前付きカラムの辞書**として表現し、エラーバーなど将来の系列追加にスキーマ変更なしで対応できるようにしています。

```bash
curl -X POST http://localhost:8001/analyze \
  -H "Content-Type: application/json" \
  -d '{
    "type": "linear_regression",
    "data": {"columns": {"x": [0,1,2,3,4], "y": [1,3,5,7,9]}}
  }'
```

- `y_error`カラムを渡すと逆分散重み付き回帰になります（外れ値の影響を誤差の大きさに応じて弱める）
- `params`に`x_log`/`y_log`（真偽値）を渡すとlog10(x)・log10(y)に対してフィットします（片対数・両対数）。非正の値を持つデータ点はフィットから除外（ログを取れないため）、`y_log`時の重み付けは誤差伝播（σ_log10(y) ≈ σ_y / (y・ln10)）で近似。有効な点が2点未満なら`insufficient_data`エラー
- 未対応の`type`やカラム欠如・長さ不一致・（対数フィットで）データ不足は`400`＋エンベロープ形式のエラーで返ります
- 新しい解析タイプを追加する場合は `backend/worker/app/analysis/` に新規ファイルを作り `@register("タイプ名")` を付けるだけで良い構造（`app/analysis/linear_regression.py`参照）

## ログイン画面

`/login`・`/signup` でメール/パスワードまたはGoogleアカウントでログイン・新規登録できます。メール/パスワードはそのまま動作しますが、Googleログインを使うには事前にSupabase側の設定が必要です。

1. Google Cloud ConsoleでOAuthクライアントID/シークレットを発行
2. Supabase Dashboard > Authentication > Sign In / Providers > Google を有効化し、上記の値を設定
3. Supabase Dashboard > Authentication > URL Configuration > Redirect URLs に `http://localhost:3000/auth/callback` を追加

未設定の間は「Googleでログイン」ボタンを押すとSupabase側のエラーがそのまま表示されますが、メール/パスワードでのログイン・新規登録・ログアウトは設定不要ですぐ使えます。
