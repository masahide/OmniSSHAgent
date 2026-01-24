不具合3
- `NewKeyRing` が型アサーションに失敗する状況でも `nil` を返さず、`agent.NewKeyring()` の結果をラップして `keyring` を必ず埋めるようになったため、非プロキシモードで `ssh-add` などを呼んでも復帰するようになりました（`pkg/sshutil/sshutil.go:221-238`）。
- `extendedKeyringWrapper` で `agent.ExtendedAgent` を満たす実装を追加し、フラグ付き署名や拡張のリクエストが来ても安全にエラーを返すので、拡張未対応の標準エージェントでもインターフェース契約が守られます（`pkg/sshutil/sshutil.go:240-255`）。
- `Signers` メソッドが再帰呼び出しではなく `k.keyring.Signers()` を使うようになり、実行時にスタックオーバーフローするバグが解消しました（`pkg/sshutil/sshutil.go:382-385`）。
- `notice` がコールバックを非同期に実行するようになって応答がブロックしにくくなりましたが、`NotifyCallback` が内部で共有状態に触れる場合は並行性の影響（競合やゴルーチンの積み重ね）がないか確認した方が安全です（`pkg/sshutil/sshutil.go:339-341`）。

提案
1. `NewKeyRing` の挙動を検証する単体テストを追加し、`k.keyring` が常に非 nil になることと `SignWithFlags`/`Extension` の返り値が想定通りであることを確認してください。
2. `NotifyCallback` の非同期化による影響が出ていないか（同じデータ構造を複数ゴルーチンが同時に触るなど）をプレースホルダのテストや適切な同期で締めておくと安心です。

テストは実行していません。

> notice がコールバックを非同期に実行するようになって応答がブロックしにくくなりましたが、NotifyCallback が内部で共有状態に触れる場合は並行性の
    影響（競合やゴルーチンの積み重ね）がないか確認した方が安全です（pkg/sshutil/sshutil.go:339-341）。　この点問題ないか確認できますか？

**確認結果**
- `KeyRing.notice` は現在 `go k.NotifyCallback(action, data)` で非同期実行されており、`App.notice` による UI/イベント処理が鍵操作スレッドをブロックしないようになっています（`pkg/sshutil/sshutil.go:353-361`）。
- `App.notice` 側を見ると、イベント系の処理はすべて読み取り専用かアウトゴーイングな呼び出しにとどまっていて、共有データを直接書き換える箇所はありません（`app.go:171-189`）。
  - `"Add/Remove/RemoveAll"` では `runtime.EventsEmit`（Wails のイベントエミッタはゴルーチンからの呼び出しを想定した API）だけを呼び、他のフィールドに触りません（`app.go:173-176`）。
  - `"Added/Removed/RemovedAll"` では `setTrayTooltip` を呼ぶだけで、そこでは `a.keyRing.KeyList()` で読み取った鍵数を用いて `TrayIcon.SetTooltip` を呼ぶだけです（`app.go:41-48`、`app.go:177-178`）。
  - `setTrayTooltip`→`KeyList`→`listPublickeys`→`List` はすべて `agent.NewKeyring()` の内部 `sync.Mutex` で保護された操作を透過しており、複数ゴルーチンから同時に呼び出しても状態破壊を起こしません（`pkg/sshutil/sshutil.go:286-295`、`pkg/sshutil/sshutil.go:252-262`、`pkg/sshutil/sshutil.go:223-230`、`C:\Users\masahide\go\pkg\mod\golang.org\x\crypto@v0.23.0\ssh\agent\keyring.go:25-33`、`C:\Users\masahide\go\pkg\mod\golang.org\x\crypto@v0.23.0\ssh\agent\keyring.go:124-131`）。
  - `TrayIcon.SetTooltip` はローカルの `notifyIconData` を使って Win32 に通知するだけなので、呼び出し元がどのスレッドでも構いません（`pkg/wintray/wintray.go:183-188`）。
  - `"Sign/SignWithFlags"` の分岐で呼ばれる `onSign` は `keyRing.FindPrivKey` を通じて鍵一覧を読み出すだけなので、こちらも並行実行の問題はありません（`app.go:180-189`、`app.go:192-200`、`pkg/sshutil/sshutil.go:416-421`）。
- `App.notice` は `a.settings` やトレイ以外の共有状態を書き換えておらず、同期的に実行されていたときと同じ安全性を保ちながらブロッキングを避けられています。

以上より、`go` を使った非同期コールバック化が追加の競合やデータ競争を招く懸念は現時点では見当たりません。

不具合５
• Goroutine管理レビュー

  - (重大) app.go:117 で context.WithCancel を作って a.cancelAgents に保持しているのに、その context は pageant.RunAgent/namedpipe.RunAgent/
    unix.RunAgent に渡されていません。shutdown 側では a.cancelAgents() を呼んだあと a.wg.Wait() (app.go:239) で各エージェント goroutine の終了を
    待っていますが、pkg/pageant/pageant.go:171 の WinMessage ループや pkg/namedpipe/namedpipe.go:35/pkg/unix/unix.go:22 の Accept() ループは
    cancel を受け取れないため解除されず、RunAgent は永久にブロックしたままです。その結果「Quit」操作で shutdown が返ってこなくなり、アプリ全体が
    終了できなくなります。実際に a.cancelAgents を RunAgent に渡してリスナーを閉じるか、wg.Wait を呼ばない形にしない限りこの Q3 の修正は逆効果で
    す。
  - (高) 本来の「接続ごとに goroutine が増える」問題は未解決です。pkg/namedpipe/namedpipe.go:35-47 や pkg/unix/unix.go:42-58 では今も go
    a.handle(conn) で毎接続ごとに goroutine を起動し、agent.ServeAgent が返るまで終了しないため接続が増えるほど goroutine が蓄積します。app.go
    の wg 管理はトップレベルの RunAgent にしか作用せず、実際の接続ハンドラは未管理なので、問題の本質的な再発が止まっていません。

  質問/想定

  - RunAgent 側で context.Context を受け取ってリスナー／ウィンドウを閉じる設計に変更する方向で進める予定でしょうか？今のところ shutdown からの
    キャンセルが agent 側に届いていないように見えます。

  次のステップ

  1. App から生成する各 RunAgent に context.Context を渡し、NamedPipe/DomainSock の Accept ループや Pageant のメッセージループでキャンセルを監視
     して listener を Close したうえで wg.Done() するようにする。「Quit」で wg.Wait を呼んでも実際に goroutine が抜けるようにしてください。
  2. 接続ハンドラ (go a.handle(conn)) の goroutine も何らかの形でライフサイクルを管理／制限し、切断処理や Context キャンセルで確実に終了させる仕
     組みを導入して bug5 の根本原因に対処してください。

  **対応済みステップ**

  - `App` が `context.WithCancel` で `agentCtx` を保持し、`RunAgent(ctx)` にその `context` を渡すことで shutdown 時に `cancelAgents()` で全エージェントをキャンセルします（`app.go`）。
  - NamedPipe/Unix/Cygwin の Accept ループを共通の `agentlistener.Serve` ヘルパーに切り出し、`ctx.Done()` でリスナーを止めつつ `wg` と接続ハンドラを終了させるようにしました（`pkg/agentlistener/listener.go`）。
  - 各接続ハンドラも context を監視して `conn.Close()` するので `agent.ServeAgent` が長時間ブロックしません（`pkg/namedpipe/namedpipe.go`、`pkg/unix/unix.go`、`pkg/cygwinsocket/cygwinsocket.go`）。
  - `app_test.go` と `pkg/agentlistener/listener_test.go` で `shutdown`→`cancelAgents`→`ctx.Done()` の流れを検証しています。

### Ctrl+C / Quit 後の確認

- `agentlistener.Serve` はキャンセル発生時に `agentlistener: listener closed due to context cancellation`、Accept 中の異常時に `agentlistener: Accept error: …` とログを出すようになったため、`Ctrl+C` / UI の `Quit` 操作で `shutdown` が呼ばれた際にそれぞれのリスナーが速やかに抜けていることをログで追えるようになっています。
- Wails UI から Quit、または CLI で `Ctrl+C` すると `App.shutdown` 内で `cancelAgents()`→`wg.Wait()` の順で処理され、`pkg/agentlistener/listener.go` の出力がすべて出たらソケットはクローズ済みと判断できます。


不具合4
• 修正レビュー

  - pkg/cygwinsocket/cygwinsocket.go:93 〜 :118 の handle 関数に defer conn.Close() を追加し、5分の SetReadDeadline も入ったので、各接続が最後ま
    で処理されたあと確実に FD を解放するようになっています。agent.ServeAgent が EOF で抜けても後続の conn.Close() が実行され、長時間運用時の FD
    枯渇リスクが解消されます。
  - main ブランチ（origin/main:pkg/cygwinsocket/cygwinsocket.go）では handle がリクエスト処理後に即 nil を返しており、conn.Close() も
    ReadDeadline のような防御策も存在しないため、接続ごとに FD が残る問題が元々ありました。今回の追加はその漏れを直接塞いでいます。

  テストは未実行です。


不具合６
• レビュー結果

  - pkg/unix/unix.go:61-68（origin/main の状態）では handle が agent.ServeAgent を呼び出すだけで読み取りタイムアウトがなく、クライアント側が送信
    を止めると goroutine が永久にブロックしてしまう状態でした。pr-64 では conn.SetReadDeadline(time.Now().Add(5 * time.Minute)) を追記し、
    agent.ServeAgent の前に読み取り期限を設けることでこのハングを防いでいます (pkg/unix/unix.go:61-66)。
  - 同様に pkg/namedpipe/namedpipe.go:51-57 でも SetReadDeadline を追加し、名前付きパイプの each-connection goroutine に５分のタイムアウトを設け
    ました。以前は agent.ServeAgent が無期限でブロックしていたため、クライアントの異常切断で goroutine や FD が寝たままになる恐れがありました。
  - pkg/cygwinsocket/cygwinsocket.go も handle 内で defer conn.Close() に加え conn.SetReadDeadline(time.Now().Add(5 * time.Minute)) を入れ、
    Cygwin ソケットでも同様に読み取りタイムアウトを確保しています (pkg/cygwinsocket/cygwinsocket.go:93-101)。

  これで各エージェント接続がクライアントの無通信や異常切断で無期限に待つことはなくなり、goroutine／FD漏れのリスクが解消されます。テストは未実施
  です。


  
