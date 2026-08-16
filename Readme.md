# AnalyseApp

![CI](https://github.com/whitegauss/analyseapp/actions/workflows/ci.yml/badge.svg)

実験データの解析（グラフ化・回帰分析など）を簡単に行うためのWebアプリ。設計の詳細は [PDR.md](./PDR.md) を参照。

## 技術スタック
- **API Gateway/BFF**: Go（[go-chi](https://github.com/go-chi/chi) + [zerolog](https://github.com/rs/zerolog)）
- **解析ワーカー**: Python（FastAPI + structlog + numpy、将来的にgRPC常駐プロセス化を予定）
- **キャッシュ**: Redis（[go-redis](https://github.com/redis/go-redis)）。解析結果を`analysis:{experiment_id}:{type}:{params_hash}`キーで24hキャッシュ（PDR.md §7）
- **DB/認証**: Supabase（PostgreSQL / Auth）。Go APIが `DATABASE_URL` で直接Postgresに接続する唯一の経路（[pgx](https://github.com/jackc/pgx)）。認証はSupabase AuthのJWT（JWKS/ES256）をGo APIが検証（[golang-jwt](https://github.com/golang-jwt/jwt) + [keyfunc](https://github.com/MicahParks/keyfunc)）
- **フロントエンド**: Next.js 16（App Router）/ React 19 / TypeScript 5 / Tailwind CSS v4 / ESLint 9（`@/` エイリアス構成）。認証は [@supabase/ssr](https://github.com/supabase/ssr) でCookieベースのセッション管理（Server Actions + `proxy.ts`でのセッションリフレッシュ）。グラフ描画は [Plotly.js](https://plotly.com/javascript/)（PDR.md §6）。軸ラベルの数式表示は [KaTeX](https://katex.org/)
- **コンテナ/開発基盤**: Docker Compose

現状はバックエンド（SupabaseのPostgres接続・スキーマ・JWT認証・experiments CRUD・Python Workerでの線形回帰解析）に加え、フロントエンドのログイン画面と実験データ入力・グラフ表示まで実装済みです。ログイン後はトップページ（`/`）で直接データ貼り付け→ライブプレビュー→保存ができ、保存後は`/experiments/{id}`で確認できます。`/experiments`では保存済みの実験を一覧できます。軸ラベルは1つのテキスト欄に`$v$`のように`$...$`で数式部分を囲んで入力すると、その部分だけKaTeXで数式（変数は斜体）として描画され、それ以外は単位・日本語も含めそのまま立体表示されます（PDR.md §6の`axis_label_runs`の考え方をベースにしたシンプル版）。`/experiments/{id}`では線形回帰の回帰直線がグラフに自動で重ね描画されます（オフにもできます）。

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
- フロントエンド: `http://localhost:3000`（ログイン中はトップページで実験データ入力、`/login`・`/signup`）
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

## テスト / Lint / CI

`main`へのpush・PRで [.github/workflows/ci.yml](./.github/workflows/ci.yml) がフロントエンド/Go API/Python Workerを並列でlint・フォーマットチェック・テスト・ビルドします。

- フロントエンド: `cd frontend && npm install`
  - テスト: `npm run test`（[Vitest](https://vitest.dev/)。`lib/pasteDataParsing.ts`の貼り付けデータパース、`components/AxisLabel.tsx`の`$...$`区切りロジック、`components/ExperimentChart.tsx`の有効数字丸めロジックなど、純粋関数のみを対象）
  - Lint: `npm run lint`（ESLint / `eslint-config-next`）
  - フォーマット: `npm run format`で整形、`npm run format:check`でCIと同じチェックのみ（[Prettier](https://prettier.io/)、`.prettierrc.json`）
  - ビルド: `npx next build`
- Go API: `cd backend/api`
  - テスト: `go build ./... && go vet ./... && go test ./...`（`internal/response`のエンベロープ整形、`internal/httpserver`のハンドラー検証ロジックをfake store（`experiments.Store`インターフェース）でテスト。DBを要する`experiments.Repository`のSQL自体は今回未カバー）
  - Lint/フォーマット: [golangci-lint](https://golangci-lint.run/)（`.golangci.yml`）。`golangci-lint run ./...`でlint、`golangci-lint fmt ./...`でフォーマット（`--diff`で差分確認のみ）
- Python Worker: `cd backend/worker && pip install -r requirements-dev.txt`
  - テスト: `pytest`（`tests/test_linear_regression.py`が解析ロジック、`tests/test_main.py`がHTTP層・エラーエンベロープをカバー）
  - Lint/フォーマット: [Ruff](https://docs.astral.sh/ruff/)（`ruff.toml`）。`ruff check .`でlint、`ruff format .`でフォーマット（`--check`で差分確認のみ）

## API（experiments）

Go API GatewayのAPI構成は [backend/api/openapi.yaml](./backend/api/openapi.yaml)（OpenAPI 3.0 / Swagger）にまとめています。[Swagger Editor](https://editor.swagger.io/) 等に貼り付けると一覧・スキーマを確認できます。

`/api/v1/experiments`系のエンドポイントは全てSupabase AuthのJWT（`Authorization: Bearer <access_token>`）が必須です。curlで試す場合、Supabase AuthのREST APIでサインアップ/サインインしてトークンを取得できます。

```bash
curl -X POST "$SUPABASE_URL/auth/v1/signup" \
  -H "apikey: $NEXT_PUBLIC_SUPABASE_ANON_KEY" -H "Content-Type: application/json" \
  -d '{"email":"you@example.com","password":"..."}'
# レスポンスの access_token を使う
```

- `POST /api/v1/experiments` — `{title, raw_data, config?}` を送信し実験を作成（`title`は省略・`null`可。空文字は`null`として保存されます）
- `GET /api/v1/experiments` — 自分が作成した実験を作成日時の新しい順に一覧取得（ページネーションなし）
- `GET /api/v1/experiments/{id}` — 自分が作成した実験を取得（他人のIDや存在しないIDは404）
- `DELETE /api/v1/experiments/{id}` — 自分が作成した実験を削除（他人のIDや存在しないIDは404）。関連する`analysis_results`行はDBの`ON DELETE CASCADE`で一緒に削除されます
- `PATCH /api/v1/experiments/{id}/config` — `{config}` でグラフ設定を丸ごと置き換え
- `PATCH /api/v1/experiments/{id}/raw_data` — `{raw_data}` でデータ本体を丸ごと置き換え。成功するとこの実験のRedisキャッシュ済み解析結果（`analysis:{experiment_id}:*`）を全て無効化します（ベストエフォート。Redisに到達できない場合も更新自体は成功します）
- `POST /api/v1/experiments/{id}/analyze` — `{type, params?}` を送信し、その実験の`raw_data`に対して解析を実行（例: `{"type":"linear_regression"}`）。Go APIが実験を取得したうえでPython Workerの`POST /analyze`に中継し、Workerのレスポンス（`{data, error, meta}`）をそのまま返します。Workerに到達できない場合は`502`（`worker_unreachable`）。成功した結果はRedisに24hキャッシュされ（`analysis:{experiment_id}:{type}:{params_hash}`、PDR.md §7）、レスポンスヘッダー`X-Cache: HIT`/`MISS`でキャッシュ命中を確認できます。キャッシュ命中時はDB・Worker呼び出し自体が発生しません。Redisに到達できない場合もキャッシュなしで通常通り動作します

## 実験データ入力・グラフ表示（フロントエンド）

ログイン後、トップページ（`/`）に実験データ入力フォームが直接表示されます（別ページへの遷移は不要）。

- タイトルは任意（未入力なら`null`で保存され、表示時は「(無題)」になります）
- スプレッドシートからのコピー＆ペースト想定（タブ／カンマ／スペース区切りを自動判定）。1列目=`x`、2列目=`y`固定
- 3列目以降は列ごとに役割を選択（`y_error` / `x_error` / 使わない / カスタム名）。Python Workerの`DataSeries.columns`と同じキー名で保存されるため、将来の解析連携にそのまま使える
- **貼り付けた瞬間にクライアント側だけでPlotly.jsのグラフがライブ更新される**（保存前のプレビュー）。データが無い間・Plotly読み込み中はスケルトン（`components/ChartSkeleton.tsx`）を表示
- 軸ラベルの入力欄はデータ貼り付け前から常時表示。1つのテキスト欄にまとめて記入し、`$...$`で囲んだ部分だけTeXの数式としてKaTeXでレンダリング（変数は自然に斜体になる）、それ以外の部分（単位や日本語など）はそのままDOM表示（立体）にする方式（例: `速度 $v$ (m/s)`）。作成フォームで設定した内容がグラフのプレビューにもそのまま反映される（説明はラベルの横の「i」アイコンにカーソルを合わせると表示）
- グラフはPlotlyのデフォルトのx=0/y=0のゼロラインを非表示にしています
- 凡例（データ・回帰直線）を枠線付きでグラフ内（左上）に常に表示します。マウスでドラッグしてグラフ内の好きな位置に移動でき、グラフ下の「凡例の文字サイズ」スライダーで文字サイズ（8〜24px）を変更できます。凡例のテキストはグラフ上でダブルクリックすると直接編集できます（回帰直線側は初期状態で回帰式`y = (a ± σa)x + (b ± σb)`のように傾き・切片双方の1σ不確かさ付きで表示。データ側は「データ」のまま）。凡例内の`$...$`で囲んだ部分は軸ラベルと同じ`$...$`記法で斜体表示されます（Plotly純正の凡例のため、軸ラベルのKaTeX表示とは異なりPlotly自体のフォントでの近似斜体になります）
- 「保存してグラフを確定」を押すとGo APIに保存され、`/experiments/{id}` にリダイレクトして確定版のグラフを表示（`y_error`/`x_error`があればエラーバー付き、軸ラベルも保存した内容で表示）
- トップページの入力フォーム・`/experiments/{id}`は未ログインだと`/login`にリダイレクトされる
- `/experiments/{id}`表示時にサーバー側で`POST /api/v1/experiments/{id}/analyze`（`type: linear_regression`）を自動実行し、回帰直線をグラフの枠いっぱいに重ね描画します（データ点の範囲だけでなくプロット領域の端まで伸ばして表示）。回帰直線の周囲には傾き・切片それぞれの±1σ標準誤差を組み合わせた不確かさの帯（薄い赤の塗りつぶし）も表示します（共分散は考慮しない簡易的な envelope で、傾き・切片それぞれの±1σの4通りの組み合わせのうち両端で最も広がる線を採用。厳密な信頼区間ではありません）。「回帰直線を表示」チェックボックスでオン/オフ可能（既定はオン、帯も連動）。傾き・切片・R²も表示。傾き・切片はそれぞれの1σ標準誤差の最初の有効数字までで丸めて表示します（例: 傾き=2.04 ± 0.05、切片の誤差も同様に表示）。標準誤差が小さい場合は小数点以下、大きい場合（十の位・百の位など）は整数側の桁でも丸められます（例: 標準誤差52.3なら値517→520）。表示する小数桁数は最大4桁までに制限しています（不確かさが非常に小さい場合でも桁数が際限なく増えないように）。標準誤差が0（近似が完全すぎて桁落ちする場合など）は`0.0`と表示します。データ不足などで解析が失敗した場合は回帰直線なしで生データのみ表示されます（ページ全体は壊れません）
- 回帰直線はトップページの保存前ライブプレビューには表示されません（解析は保存済み実験の`raw_data`に対してのみ実行されるため）
- `/experiments`で保存済みの実験を一覧できます（作成日時の新しい順、タイトルと作成日を表示、クリックで`/experiments/{id}`へ）。トップページの「実験データを追加」見出し横にリンクがあります。1件も無い場合は空状態メッセージを表示
- 一覧の各行に「削除」ボタンがあり、押すとその場でインラインの確認（「削除する」/「キャンセル」）に切り替わります（ブラウザのalert/confirmダイアログは使用しません）。削除すると`/experiments`に戻り一覧から消えます
- `/experiments/{id}`の「軸ラベルを編集」ボタンから軸ラベル（X軸・Y軸）を事後編集できます（インラインフォーム、`PATCH /api/v1/experiments/{id}/config`を呼び出し）。保存すると同じページを再読み込みし、更新後の内容がグラフに反映されます
- `/experiments/{id}`の「データを編集」ボタンからも同様にデータ本体（`raw_data`）を事後編集できます。作成時と同じスプレッドシート貼り付け形式（テキストエリア＋3列目以降の役割選択）で、現在のデータを再構成した状態で編集を開始できます（`PATCH /api/v1/experiments/{id}/raw_data`を呼び出し、`raw_data`は丸ごと置き換え）。データを編集すると、Redisにキャッシュされていたこの実験の解析結果（回帰直線など）は自動的に無効化され、次回表示時に再計算されます

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
- 未対応の`type`やカラム欠如・長さ不一致は`400`＋エンベロープ形式のエラーで返ります
- 新しい解析タイプを追加する場合は `backend/worker/app/analysis/` に新規ファイルを作り `@register("タイプ名")` を付けるだけで良い構造（`app/analysis/linear_regression.py`参照）

## ログイン画面

`/login`・`/signup` でメール/パスワードまたはGoogleアカウントでログイン・新規登録できます。メール/パスワードはそのまま動作しますが、Googleログインを使うには事前にSupabase側の設定が必要です。

1. Google Cloud ConsoleでOAuthクライアントID/シークレットを発行
2. Supabase Dashboard > Authentication > Sign In / Providers > Google を有効化し、上記の値を設定
3. Supabase Dashboard > Authentication > URL Configuration > Redirect URLs に `http://localhost:3000/auth/callback` を追加

未設定の間は「Googleでログイン」ボタンを押すとSupabase側のエラーがそのまま表示されますが、メール/パスワードでのログイン・新規登録・ログアウトは設定不要ですぐ使えます。
