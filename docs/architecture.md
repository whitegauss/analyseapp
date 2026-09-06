# アーキテクチャ

コードがどう分かれていて、なぜそう分かれているか。設計の意図は [PDR.md](../PDR.md)、動かし方は [Readme.md](../Readme.md)。

## 全体像

3つのサービスが1つの repo に入っている。

```
ブラウザ
  │
  ▼
frontend/          Next.js 16 (App Router)
  │                サーバー側だけが Go API を呼ぶ。ブラウザは直接叩かない
  ▼
backend/api/       Go — API Gateway / BFF
  ├──▶ Supabase Postgres     データの唯一の保存先
  ├──▶ Redis                 解析結果のキャッシュ（best-effort）
  └──▶ backend/worker/       Python — 数値解析だけを担う
```

**ワーカーはクライアントから到達できない。** Go API だけが呼ぶ内部サービス。認証も DB も持たない。

## backend/api（Go）

### パッケージの役割

| パッケージ                                 | 責務                                                       | I/O       |
| ------------------------------------------ | ---------------------------------------------------------- | --------- |
| `cmd/api`                                  | 起動。設定を読み、依存を組み立てて `NewRouter` に渡す      | —         |
| `cmd/migrate`                              | マイグレーション適用（compose の `tools` プロファイル）    | Postgres  |
| `internal/httpserver`                      | ルーティング、ハンドラー、検証、エラー写像                 | —         |
| `internal/auth`                            | Supabase JWT の検証（**`/api/v1` 全体の唯一の認可地点**）  | JWKS 取得 |
| `internal/experiments` `internal/projects` | SQL を持つリポジトリ + `Store` インターフェース            | Postgres  |
| `internal/cache`                           | キー導出 + `Cache` インターフェース + Redis 実装           | Redis     |
| `internal/worker`                          | ワーカーへの HTTP クライアント + `Client` インターフェース | HTTP      |
| `internal/response`                        | `{data, error, meta}` エンベロープの整形 + `StatusWriter`  | —         |
| `internal/logging` `internal/metrics`      | トレース ID、構造化ログ、Prometheus                        | stdout    |
| `internal/config`                          | 環境変数 → `Config`                                        | —         |

### 依存を注入する境界（テストが成立している理由）

ハンドラーは**具体型ではなくインターフェース**を受け取る。

```go
func handleCreateExperiment(repo experiments.Store) http.HandlerFunc
func NewRouter(dbPool *pgxpool.Pool, jwks keyfunc.Keyfunc,
                workerClient worker.Client, resultCache cache.Cache) http.Handler
```

インターフェースは4つ。**これがテストの継ぎ目**で、ハンドラーのテストが DB も Redis もワーカーも立てずに回るのはこの形のおかげ。

| インターフェース    | 定義場所                                 | 本番実装             |
| ------------------- | ---------------------------------------- | -------------------- |
| `experiments.Store` | `internal/experiments/experiments.go:34` | `*Repository`（pgx） |
| `projects.Store`    | `internal/projects/projects.go:35`       | `*Repository`（pgx） |
| `cache.Cache`       | `internal/cache/cache.go:21`             | `*RedisCache`        |
| `worker.Client`     | `internal/worker/client.go:17`           | `*HTTPClient`        |

**新しい I/O を足すときは、まずインターフェースを切ってからハンドラーに渡すこと。** 具体型を直接持たせると、その瞬間からハンドラーのテストに実インフラが要るようになる。

### ミドルウェアの順序（`router.go`）

外側から:

```
Recoverer → logging → metrics → securityHeaders → ClientIPFromRemoteAddr
  → [/api/v1 のみ] httprate → auth
```

**この順序には既知の問題がある。** `Recoverer` が `logging` / `metrics` より外側にあり、両者が記録を `defer` せずに行っているため、**panic したリクエストはメトリクスにもログにも残らない**（KAN-67）。触るときは注意すること。

### 認可

`auth.Middleware` が `/api/v1` 配下に一括で掛かる。ここを通ったリクエストだけが context に `user_id` を持つ。

リポジトリの SQL は必ず `where ... and user_id = $N` で絞る。`ErrNotFound` は「存在しない」と「他人のもの」を**意図的に区別しない** — 区別すると他人のデータの存在が漏れる。

## backend/worker（Python）

FastAPI の**同期 HTTP サービス**。キューでもワーカープールでもない。

解析タイプは `app/analysis/` のレジストリで引く。新しい解析を足すときは、ファイルを作って `@register("タイプ名")` を付けるだけ。`app/analysis/linear_regression.py` が唯一の実例。

**描画はしない。** 数値だけを返し、グラフはクライアントごとに描く（PDR §6）。

## frontend（Next.js）

### データ取得はサーバー側に閉じている

**ブラウザから Go API を直接叩く経路は無い。** すべて `lib/api.ts` の `callGoApi` を通り、その呼び出し元はサーバーコンポーネント・サーバーアクション・ルートハンドラーのいずれか。

```
Server Component / Server Action / Route Handler
   └─▶ lib/api.ts callGoApi   Supabase セッションから JWT を載せる
          └─▶ Go API
```

クライアントコンポーネントがサーバーのデータを要るときは、Server Action を呼ぶ（`ExperimentChart` が対数軸の回帰を取りに行く経路がこれ）。

### `lib/` と `components/` の依存の向き

**`lib/` は `components/` を import しない。** 一方向。

```
app/  ──▶ components/ ──▶ lib/
  └────────────────────────▲
```

理由は、`app/` 配下のサーバーコードが型やロジックを名指しするためだけに `"use client"` のモジュールを読み込む形になっていたのを解消したため。ドメインの型（`LinearRegressionResult` など）とロジックは `lib/` に置く。

`lib/` の中身:

| ディレクトリ    | 中身                                                                   |
| --------------- | ---------------------------------------------------------------------- |
| `lib/*.ts`      | ドメインの型、パース、単位換算、有効数字、統計、CSV                    |
| `lib/chart/`    | グラフの計算（対数目盛り、軸レンジ、回帰の評価、±1σ 帯、凡例テキスト） |
| `lib/supabase/` | Supabase クライアントの生成（唯一の I/O）                              |

**`lib/` に置くものは純粋関数**（`lib/api.ts` と `lib/supabase/` を除く）。コンポーネントはそれを呼ぶだけにする。この分離が、コンポーネントを描画せずにロジックを検証できる理由。

### `lib/` に何を出すかの判断

コンポーネントの中に「入力から出力が決まるだけの計算」があれば `lib/` の候補。特に:

- `useMemo` の中に書かれた計算 — **描画しないと触れないので見落としやすい**
- 2つ以上のコンポーネントが同じことをしている
- サーバー側のコードが欲しがっている型やロジック

## 既知の穴

実装済みだが直っていないもの。テストで現状を固定してあるので、直すとテストが赤くなる（[testing.md](testing.md#現状固定テスト)）。

| 課題   | 内容                                                       |
| ------ | ---------------------------------------------------------- |
| KAN-53 | `SUPABASE_URL` が誤っていても API が起動する               |
| KAN-57 | 縮退した入力で worker が envelope 無しの 500 を返す        |
| KAN-58 | `y` に NaN が混ざると `r_squared` が 1.0 になる            |
| KAN-61 | `parseColumnsField` が形状を検証しない                     |
| KAN-62 | `WORKER_BASE_URL` が起動時に検証されない                   |
| KAN-63 | 応答の読み取り失敗がステータスコードを捨てる               |
| KAN-64 | `worker.HTTPClient` を `HTTP` 未設定で構築すると panic     |
| KAN-66 | センチネル比較が `==` で、ラップされると 404 が 500 になる |
| KAN-67 | **panic したリクエストがメトリクスにもログにも残らない**   |
| KAN-68 | `DeleteByPrefix` が空プレフィックスで Redis を全消しする   |

## まだ無いもの

`PDR.md` に書いてあるが未実装。**あると思って書き始めないこと。**

- `experiments.project_id`（プロジェクト機能はバックエンドのみで、フロントの UI は無い）
- `POST /api/v1/experiments/{id}/copy`、`POST /api/v1/convert`
- API キー認証（`X-API-Key`）、監査ログ
- デプロイ経路（GHCR / k3s / Terraform / ArgoCD）
- Prometheus をスクレイプする側
