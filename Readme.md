# AnalyseApp

## 技術スタック
- **コンテナ/開発基盤**: Docker Compose (Node 18 Alpine イメージ + Python ベースコンテナ)
- **フロントエンド**: Next.js 16 (App Router) / React 19 / TypeScript 5 / Tailwind CSS v4 / ESLint 9（`@/` エイリアス構成）
- **バックエンド**: Django + Django REST framework（`django-cors-headers` で CORS 制御、`numpy` 利用）

## 使用方法
1. Docker と Docker Compose が利用できる環境を用意します。
2. 依存イメージを取得・ビルドするため `docker compose pull && docker compose build` を実行します。
3. 下記の起動方法に従ってサービスを立ち上げ、ブラウザーからフロントエンドにアクセスします。

## 起動方法
### 全サービスをまとめて起動
```bash
docker compose up
```
- フロントエンド: `http://localhost:3000`
- バックエンド API: `http://localhost:8000`

### 個別サービスの再起動例
- フロントエンドだけ: `docker compose restart frontend`
- バックエンドだけ: `docker compose restart backend`

### 終了
```bash
docker compose down
```
