# AnalyseApp

実験データの解析（グラフ化・回帰分析など）を簡単に行うためのWebアプリ。設計の詳細は [PDR.md](./PDR.md) を参照。

## 技術スタック
- **API Gateway/BFF**: Go（[go-chi](https://github.com/go-chi/chi) + [zerolog](https://github.com/rs/zerolog)）
- **解析ワーカー**: Python（FastAPI + structlog、将来的にgRPC常駐プロセス化を予定）
- **キャッシュ**: Redis
- **DB/認証**: Supabase（PostgreSQL / OAuth）※現時点では未接続（環境変数の枠のみ用意）
- **フロントエンド**: Next.js（未着手）
- **コンテナ/開発基盤**: Docker Compose

現状はバックエンドの最小スケルトン（Go API・Python Worker・Redisの疎通確認まで）のみが実装されています。experiments CRUDや解析機能、Supabase/Redisの実接続、フロントエンドは今後追加していきます。

## 使用方法
1. Docker と Docker Compose が利用できる環境を用意します。
2. リポジトリ直下の `.env.example` を `.env` にコピーし、必要に応じて値を設定します（Supabase未接続の間は空のままで動作します）。
3. 下記の起動方法に従ってサービスを立ち上げます。

## 起動方法
### 全サービスをまとめて起動
```bash
docker compose up --build
```
- Go API: `http://localhost:8080`（ヘルスチェック: `GET /healthz`）
- Python Worker: `http://localhost:8001`（ヘルスチェック: `GET /healthz`）
- Redis: `localhost:6379`

### 個別サービスの再起動例
- APIだけ: `docker compose restart api`
- Workerだけ: `docker compose restart worker`

### 終了
```bash
docker compose down
```
