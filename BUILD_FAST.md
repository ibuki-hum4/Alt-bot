# Build Fast Memo (Alt-bot)

最終更新: 2026-04-19

## まずこれだけ

開発中の通常ビルド:

```powershell
powershell -NoProfile -File .\compose-fast.ps1 -Action build
```

起動まで一気に:

```powershell
powershell -NoProfile -File .\compose-fast.ps1 -Action up
```

キャッシュ無効でフル再ビルド:

```powershell
powershell -NoProfile -File .\compose-fast.ps1 -Action build -NoCache
```

## 重要メモ

- この環境では `pwsh` コマンドが見つからないことがある。`powershell` を使う。
- `-File compose-fast.ps1` で失敗したら `-File .\\compose-fast.ps1` のように相対パスを付ける。
- `-NoCache:$false` は不要。`-NoCache` は付けるか付けないかで使う。
- ビルドの主ボトルネックは Go コンパイル (`RUN go build`)。コンテキスト転送量は既に小さい。

## 反映済みの高速化

- `.dockerignore` 追加済み: ローカル成果物を除外
- Dockerfile 改善済み:
  - `COPY . .` を廃止し、必要ディレクトリのみ COPY
  - `go build -buildvcs=false` を適用
- 補助スクリプト追加済み: `scripts/compose-fast.ps1`

## 今回の実測

- `podman compose -f docker_compose.yaml build bot`: 約 238 秒
- `powershell -NoProfile -File .\scripts\compose-fast.ps1 -Action build`: 約 221 秒

## さらに速くしたい場合

1. 開発時は DB だけコンテナ起動し、bot はホストで `go run` 実行
2. 依存更新がないときは `-NoCache` を使わない
3. 長時間ビルド時は差分が少ないうちにこまめにビルドしてキャッシュを維持
