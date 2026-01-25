# Win32 通知領域アイコンと goroutine のスレッド問題

## 現象

* 通知領域アイコンが表示されない、表示が不安定、右クリックメニューが出ない、メッセージ処理が止まる、終了できないなどが発生する
* 特に次の構成で起きやすい

  * トレイ初期化やウィンドウ生成を goroutine で起動した
  * メッセージループを goroutine に追い出した
  * 別 goroutine から Win32 の UI 系 API を呼び出した

## 何が起きているか

Win32 の通知領域アイコンは概ね次の 3 点に依存する。

1. 隠しウィンドウの存在

   * `Shell_NotifyIcon` は `NOTIFYICONDATA.hWnd` を通じて、通知アイコンに紐づくウィンドウへコールバックメッセージを配送する
   * このウィンドウはスレッドに所属する

2. スレッドのメッセージキューとメッセージループ

   * `GetMessage` `PeekMessage` `DispatchMessage` などで回すメッセージループが必要
   * これが止まる、または別スレッドで回るとコールバックが処理できない

3. スレッドアフィニティの破綻

   * Go の goroutine は実行 OS スレッドが固定されない
   * そのため、ウィンドウ生成や `Shell_NotifyIcon` 実行時の OS スレッドと、メッセージループ実行時の OS スレッドが一致しない状態が発生し得る
   * 一致しないと、アイコンが出ない、コールバックが届かない、UI が固まるなどが起きる

## 問題の本質

* 通知領域アイコンが依存する「隠しウィンドウ」と「メッセージループ」が、同一 OS スレッド上で生存し続ける保証が崩れている
* goroutine を安易に使うと Win32 側のスレッド前提を破壊し、結果として次の不整合が起きる

  * ウィンドウを作ったスレッドとメッセージループを回しているスレッドが一致しない
  * コールバックが届かない、または届いても処理されない
  * UI が固まる、終了できない

## LockOSThread に関する重要な整理

### 誤解しやすい点

* `runtime.LockOSThread()` はプロセス全体を単一スレッドに固定しない
* 固定されるのは「呼び出した goroutine が実行される OS スレッド」だけである
* ほかの goroutine は通常どおり別スレッドで並行並列実行される

### 根拠

* Go 公式 runtime ドキュメントは「calling goroutine」を主語に説明しており、呼び出した goroutine を OS スレッドへ紐づける機能であることが明示されている

  * さらに「そのスレッドでは、呼び出した goroutine 以外は実行されない」と明記されている
  * これは裏返すと、他の goroutine は別スレッドで通常動作することを前提とする
    参照: Go runtime docs (`runtime.LockOSThread`)
    [https://pkg.go.dev/runtime](https://pkg.go.dev/runtime)

* Go Wiki でも、UI ループを固定スレッドで回しつつ、それ以外の処理は別 goroutine に出す定石が説明されている
  参照: Go Wiki LockOSThread
  [https://go.dev/wiki/LockOSThread](https://go.dev/wiki/LockOSThread)

## よくある誤りパターン

* `go initTray()` のようにトレイ初期化を別 goroutine で実行し、そこでウィンドウ生成や `Shell_NotifyIcon` を叩く
* `go messageLoop()` のようにメッセージループを別 goroutine で実行し、ウィンドウ生成スレッドと一致しない
* トレイの更新や削除を別 goroutine から直接 `Shell_NotifyIcon` で実行する
* main が先に終了してしまう、またはメッセージループを止めてしまう

## 取るべき対策

### 方針

トレイ処理を「専用 UI スレッド 1 本」に集約する。そこで OS スレッドを固定し、隠しウィンドウとメッセージループを同じ OS スレッドで完結させる。
本体処理は別 goroutine 群で通常どおり並行並列に動かす。

### 必須対応

1. トレイ専用 goroutine を作り、その goroutine 内で `runtime.LockOSThread()` を呼ぶ

   * ウィンドウ生成より前に Lock する
   * Lock した goroutine は UI とメッセージ処理専用にする

2. 隠しウィンドウ生成とメッセージループを同一スレッドに閉じ込める

   * 典型フロー

     * `RegisterClassEx`
     * `CreateWindowEx` または `CreateWindow`
     * `Shell_NotifyIcon(NIM_ADD)`
     * `GetMessage/DispatchMessage` でループ

3. 他 goroutine からの Win32 UI 呼び出しを禁止する

   * `Shell_NotifyIcon` の追加、更新、削除
   * `DestroyWindow`
   * メニュー表示
   * これらはすべてトレイスレッドでのみ実行する

### 推奨アーキテクチャ

* 本体処理からトレイスレッドへはコマンドキューで依頼する

  * `chan TrayCommand` のような Go チャネル
  * 併用として `PostMessage` を隠しウィンドウへ送って起床させる
* トレイスレッドは Win32 メッセージ処理を止めないことを最優先とする

  * メッセージループ中に重い処理をしない
  * 重い処理は本体 goroutine 側で行い、結果だけトレイスレッドへ渡す

## LockOSThread 利用時の注意点

* Lock した goroutine は、その OS スレッドを専有する

  * これは設計上のコストであり、トレイ用途なら通常許容される
* 問題が起きるのは次のとき

  * Lock した goroutine 内で重い処理やブロッキング I/O を抱える
  * UI 以外の仕事までトレイスレッドへ寄せてしまう
* 対策

  * トレイスレッドは UI とメッセージ処理のみ
  * ネットワーク、ディスク I/O、計算は別 goroutine に逃がす

参照: Go Wiki LockOSThread
[https://go.dev/wiki/LockOSThread](https://go.dev/wiki/LockOSThread)

## 実装タスクに落とすと

* トレイ処理を `TrayThread` 的なコンポーネントに分離する
* `TrayThread.Start()` は goroutine で起動してよいが、起動した goroutine 内で `runtime.LockOSThread()` して以降はその OS スレッドで次を実行する

  * hidden window 作成
  * `Shell_NotifyIcon` 登録
  * メッセージループ
* トレイ更新 API を `TrayThread.Post(cmd)` に統一し、Win32 直呼びを禁止する
* 終了処理は UI スレッドで完結させる

  * `Shell_NotifyIcon(NIM_DELETE)` を必ず実行
  * `PostQuitMessage` などでメッセージループを抜ける
  * 必要なら `WaitGroup` で UI スレッド終了を待つ

## 受け入れ基準

* トレイアイコンが安定して表示される
* 左右クリックなどのイベントが確実に処理される
* トレイの更新と削除が確実に成功する
* シャットダウン時に goroutine や OS リソースがリークしない
* main のライフサイクルに依存せず、トレイスレッドが適切に生存し続ける
* 本体処理の並行並列性が維持される

## 簡易診断チェックリスト

* `Shell_NotifyIcon(NIM_ADD)` を呼んだスレッドと `GetMessage` を回しているスレッドは同じか
* `runtime.LockOSThread()` を呼ぶ位置は、ウィンドウ生成より前か
* Win32 UI 系 API を別 goroutine から叩いていないか
* メッセージループが止まっていないか
* トレイスレッド内で重い処理をしていないか

