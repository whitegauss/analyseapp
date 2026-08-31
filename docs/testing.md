# テスト

この repo でテストを書くときの約束事。実行方法だけ知りたいなら [Readme.md の「テスト / Lint / CI」](../Readme.md#テスト--lint--ci)。

## 走らせる

```bash
scripts/test.sh              # 3スタック全部。成功すると1スタック1行
scripts/test.sh api          # 単一（all | frontend | api | worker）
scripts/test.sh --cov        # カバレッジ付き
scripts/test.sh --full       # 失敗時の出力を省略しない
```

失敗したときは**失敗したスイートの出力だけ**が出る（既定で末尾80行、`MAX_FAIL_LINES` で変更可）。全文は常に `.test-logs/<suite>.log` に残る。CI も同じスクリプトを呼ぶので、手元と CI で実行方法が食い違わない。

## 基本方針: ロジックを I/O から引き剥がしてからテストする

**ハンドラーやコンポーネントを動かさないと触れない計算は、そこにある限りテストされない。**

順序はこう:

1. 入力から出力が決まるだけの計算を、純粋関数として外に出す
2. その関数を表駆動で網羅する
3. 残った I/O は、境界のインターフェース / フェイクで薄く検証する

### Go

ハンドラーは[インターフェース経由で依存を受け取る](architecture.md#依存を注入する境界テストが成立している理由)ので、DB も Redis も立てずに回る。フェイクは `backend/api/internal/httpserver/fakes_test.go` にある。

検証やエラー写像は `backend/api/internal/httpserver/errors.go` / `backend/api/internal/httpserver/validate.go` に純粋関数として出してある。**ハンドラー経由でしか踏めない分岐を作らないこと** — `writeExperimentError` はかつてカバレッジ 75% で、それは「500 になる分岐が一度も実行されていない」という意味だった。

### フロントエンド

計算は `lib/` に置き、コンポーネントは呼ぶだけにする（[依存の向き](architecture.md#lib-と-components-の依存の向き)）。

**`useMemo` の中に書かれた計算は特に見落としやすい。** 実例: 回帰の ±1σ 帯を求める `boundsAt` は `regressionBand` の `useMemo` の中にインラインで書かれていて、コンポーネントを描画しない限り触れなかった。物理実験の誤差帯という、間違えても画面上は「それっぽく」見えてしまう計算がテスト不能な位置にあった。

### Python

解析ロジック（`app/analysis/`）は素の関数なのでそのまま呼べる。HTTP 層は `fastapi.testclient.TestClient`。

## 使う道具

**入れない**もの: testify、Jest、Playwright、スナップショットテスト。

| 領域     | 道具                                                                      |
| -------- | ------------------------------------------------------------------------- |
| Go       | 標準 `testing` + `net/http/httptest`。Redis は `miniredis`                |
| フロント | Vitest。UI は今後 jsdom + Testing Library + Storybook の `composeStories` |
| Python   | pytest                                                                    |

## 現状固定テスト

**この repo でいちばん誤解されやすい約束事。**

バグを見つけても、テスト整備の PR では**直さない**。代わりに:

1. **いまの壊れた挙動**をそのまま assert するテストを書く
2. 「これは是認ではなく固定である」ことと、**課題キー**をコメントに書く
3. 修正は別課題として起票する

理由は2つ。テストを足す PR に挙動変更が混ざるとレビューの前提が壊れること。そして**バグを直したときに、直ったことがテストで分かる**こと。

### 書き方

```go
// Current behaviour, pinned rather than endorsed. HTTPClient's fields are
// exported, so a caller can build one without HTTP and Analyze then
// dereferences nil. ... Giving it a real error is KAN-64.
func TestAnalyzePanicsWhenHTTPClientIsMissing(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Analyze returned normally; it panics today, so KAN-64 may be fixed")
		}
	}()
	...
}
```

```ts
// Current behaviour, pinned rather than endorsed. An empty array is truthy,
// so a column with no data passes the check. Fixing this is KAN-61.
it.each([...])("today accepts a column map where %s", ...)
```

**必ず課題キーを書くこと。** これが無いと、バグを直した人が赤いテストを見て「自分の修正が壊した」と判断してしまう。

### バグを直すとき

固定テストが落ちたら、**それは成功のしるし**。修正を戻すのではなく、期待値を「あるべき姿」に反転させる。どのテストが落ちるかは各課題の「追加すべきテスト」に書いてある。

現状固定テストの一覧は [architecture.md の「既知の穴」](architecture.md#既知の穴)。

## リファクタが挙動を変えていないことの示し方

ロジックを `lib/` や別パッケージへ移すとき、「テストが緑」だけでは弱い。**移す前の実装をコピーして、同じ入力に対する出力が一致することを機械的に確かめる**使い捨てテストを書くと確実。

実例（KAN-48）: 抽出前のコードを逐語コピーし、**80通りの回帰**（slope/intercept 5通り × stderr 4通り × x_log × y_log）**× x 値5点**で `evaluateModel` と `uncertaintyBoundsAt` の出力が一致することを確認した。確認後に削除する（本体が消えたら比較対象も無いため）。

## 実際に動かしてから書く

**「こうあるべき」でテストを書かない。** 先に呼んで、返ってきたものを見る。

この repo で実際に踏んだ例:

- 「`pytest` は CWD 依存で壊れる」と見立てたが、**どこから実行しても通った**
- 「log フラグ無しで2点未満は numpy 例外」と見立てたが、**常に 400 だった**
- 「`y_log` と負値」は**すでにテスト済み**だった
- `encodeURIComponent` が `'` `(` `)` `*` を残すこと、`NewJWKS` が到達不能を検知しないこと、`DeleteByPrefix("")` が DB を全消しすることは、**いずれも呼んでみて初めて分かった**

推測で書いたテストは、実装のバグではなくテスト側の思い込みを固定してしまう。

## カバレッジ

```bash
scripts/test.sh --cov
```

分母は**正直に**測る。既定のままだと「テストが読み込んだファイル」しか分母に入らず、数字が実態より良く出る。

- Go: `-coverpkg=./...` で、自前のテストを持たないパッケージも分母に入れる
- Vitest: `coverage.include` で `app` / `components` / `lib` の全ファイルを対象にする（これが無いとフロントは 46% と出るが、実際は 22% だった）

**閾値ゲートは設けていない。** KAN-36（テスト基盤の再構築）の作業中は数値が動き続けるため。完了後に設定する。

## PR の分け方

1課題 = 1ブランチ = 1PR、**1 PR は 200〜300 追加行**。超えるなら分ける。

分けるときは行数で機械的に切らず、**各 PR 単体でビルドが通りテストが緑になる**形にする。土台（抽出・共通化）を先、それを使う側を後。

移動が主体で超過する場合は、**PR 本文の冒頭に「N行は逐語移動、実質レビューが必要なのは M行」と書く**。レビュアーがどこを読めばよいか分かる。
