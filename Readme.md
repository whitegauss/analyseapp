# AnalyseApp

実験データの解析（グラフ化・回帰分析など）を簡単に行うためのWebアプリ。設計の詳細は [PDR.md](./PDR.md) を参照。

## 技術スタック
- **API Gateway/BFF**: Go（[go-chi](https://github.com/go-chi/chi) + [zerolog](https://github.com/rs/zerolog)）
- **解析ワーカー**: Python（FastAPI + structlog + numpy、将来的にgRPC常駐プロセス化を予定）
- **キャッシュ**: Redis
- **DB/認証**: Supabase（PostgreSQL / Auth）。Go APIが `DATABASE_URL` で直接Postgresに接続する唯一の経路（[pgx](https://github.com/jackc/pgx)）。認証はSupabase AuthのJWT（JWKS/ES256）をGo APIが検証（[golang-jwt](https://github.com/golang-jwt/jwt) + [keyfunc](https://github.com/MicahParks/keyfunc)）
- **フロントエンド**: Next.js 16（App Router）/ React 19 / TypeScript 5 / Tailwind CSS v4 / ESLint 9（`@/` エイリアス構成）。認証は [@supabase/ssr](https://github.com/supabase/ssr) でCookieベースのセッション管理（Server Actions + `proxy.ts`でのセッションリフレッシュ）。グラフ描画は [Plotly.js](https://plotly.com/javascript/)（PDR.md §6）。軸ラベルの数式表示は [KaTeX](https://katex.org/)
- **コンテナ/開発基盤**: Docker Compose

現状はバックエンド（SupabaseのPostgres接続・スキーマ・JWT認証・experiments CRUD・Python Workerでの線形回帰解析）に加え、フロントエンドのログイン画面と実験データ入力・グラフ表示まで実装済みです。ログイン後はトップページ（`/`）で直接データ貼り付け→ライブプレビュー→保存ができ、保存後は`/experiments/{id}`で確認できます。軸ラベルは1つのテキスト欄に`$v$`のように`$...$`で数式部分を囲んで入力すると、その部分だけKaTeXで数式（変数は斜体）として描画され、それ以外は単位・日本語も含めそのまま立体表示されます（PDR.md §6の`axis_label_runs`の考え方をベースにしたシンプル版）。解析結果（回帰直線など）のグラフ重ね描画、experiments一覧UI、Go↔Worker間の解析連携（`POST /api/v1/experiments/{id}/analyze`）は今後追加していきます。

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
- `GET /api/v1/experiments/{id}` — 自分が作成した実験を取得（他人のIDや存在しないIDは404）
- `PATCH /api/v1/experiments/{id}/config` — `{config}` でグラフ設定を丸ごと置き換え

## 実験データ入力・グラフ表示（フロントエンド）

ログイン後、トップページ（`/`）に実験データ入力フォームが直接表示されます（別ページへの遷移は不要）。

- タイトルは任意（未入力なら`null`で保存され、表示時は「(無題)」になります）
- スプレッドシートからのコピー＆ペースト想定（タブ／カンマ／スペース区切りを自動判定）。1列目=`x`、2列目=`y`固定
- 3列目以降は列ごとに役割を選択（`y_error` / `x_error` / 使わない / カスタム名）。Python Workerの`DataSeries.columns`と同じキー名で保存されるため、将来の解析連携にそのまま使える
- **貼り付けた瞬間にクライアント側だけでPlotly.jsのグラフがライブ更新される**（保存前のプレビュー）。データが無い間・Plotly読み込み中はスケルトン（`components/ChartSkeleton.tsx`）を表示
- 軸ラベルの入力欄はデータ貼り付け前から常時表示。1つのテキスト欄にまとめて記入し、`$...$`で囲んだ部分だけTeXの数式としてKaTeXでレンダリング（変数は自然に斜体になる）、それ以外の部分（単位や日本語など）はそのままDOM表示（立体）にする方式（例: `速度 $v$ (m/s)`）。作成フォームで設定した内容がグラフのプレビューにもそのまま反映される（説明はラベルの横の「i」アイコンにカーソルを合わせると表示）
- グラフはPlotlyのデフォルトのx=0/y=0のゼロラインを非表示にしています
- 「保存してグラフを確定」を押すとGo APIに保存され、`/experiments/{id}` にリダイレクトして確定版のグラフを表示（`y_error`/`x_error`があればエラーバー付き、軸ラベルも保存した内容で表示）
- トップページの入力フォーム・`/experiments/{id}`は未ログインだと`/login`にリダイレクトされる
- 現状は生データのプロットのみで、回帰直線などの解析結果表示は未実装（Go側に`/analyze`中継エンドポイントを作った後に対応予定）
- 保存済み実験（`/experiments/{id}`）のデータ自体・軸ラベルの事後編集は未実装（作成時に設定した内容のみ。Go側にPATCH `/config`は既にあるが、フロントの編集UIをまだ`/experiments/{id}`に用意していない）

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
