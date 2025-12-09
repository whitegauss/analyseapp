FROM python:3.11-slim

WORKDIR /app

# 依存関係のインストール
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# コードをコピー（後でマウントしますが、ビルド用に入れておきます）
COPY . .

# サーバー起動コマンド
CMD ["python", "manage.py", "runserver", "0.0.0.0:8000"]