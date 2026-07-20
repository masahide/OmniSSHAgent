# Win Tray Sample

Go標準ライブラリと `golang.org/x/sys/windows` だけを利用し、Win32 APIを直接呼び出してWindows通知領域に常駐するサンプルです。

この版では、Windows 11の隠しアイコンパネルよりメニューが背面に表示される問題に対して、`fyne-io/systray` のWindows実装と同じ構成へ寄せています。

## fyne-io/systrayを参考に変更した点

- 通常の `WS_OVERLAPPEDWINDOW` を作成してから `SW_HIDE` で非表示化
- コールバックメッセージに `WM_USER + 1` を使用
- `NIM_SETVERSION` を呼ばず、従来形式の通知領域コールバックを使用
- `lParam` 全体を `WM_LBUTTONUP` または `WM_RBUTTONUP` として処理
- メニュー表示直前に所有ウィンドウへ `SetForegroundWindow` を実行
- `TrackPopupMenu` は `TPM_BOTTOMALIGN | TPM_LEFTALIGN` だけで呼び出し
- `TPM_RETURNCMD` と `TPM_NONOTIFY` を使わず、選択結果を `WM_COMMAND` で処理
- 独自の最前面ポップアップウィンドウや `HWND_TOPMOST` 制御を廃止

重要なのは、単に `TPM_BOTTOMALIGN` を追加するのではなく、メニューの所有ウィンドウ、通知領域のコールバック形式、コマンド配送方式まで `fyne-io/systray` と同じ形へ揃えた点です。

## 実装内容

- `Shell_NotifyIconW` による通知領域アイコンの追加、更新、削除
- 非表示トップレベルウィンドウとWin32メッセージループ
- 左クリックまたは右クリックによるコンテキストメニュー表示
- `Show notification` によるWindows通知の表示
- `Show alert dialog` によるメッセージボックス表示
- `About` ダイアログ
- `Quit` による正常終了
- Explorer再起動後の `TaskbarCreated` を受けたアイコン再登録
- `go:embed` によるICOファイルの実行ファイル内への埋め込み
- CGO不使用

## ビルド

PowerShellで次を実行します。

```powershell
go mod tidy
go build -trimpath -ldflags="-H=windowsgui" -o win-tray-sample.exe .
```

デバッグ時は `-H=windowsgui` を外すと標準出力を確認できます。

```powershell
go build -trimpath -o win-tray-sample.exe .
```

## 実行と操作

```powershell
.\win-tray-sample.exe
```

通知領域のアイコンを左クリックまたは右クリックすると、次のメニューを表示します。

- `Show notification`
- `Show alert dialog`
- `About`
- `Quit`

Windows通知は、Windowsの通知設定や集中モードによって表示されない場合があります。`Show alert dialog` は動作確認用の `MessageBoxW` です。

## アイコンの差し替え

`assets/tray.ico` を任意のICOファイルへ置き換えて再ビルドしてください。複数サイズを含むICOを推奨します。

トレイ表示用ICOは `go:embed` で実行ファイルへ格納し、一時ファイルへ展開せず `CreateIconFromResourceEx` で `HICON` を生成します。

## ファイル構成

```text
.
├── assets
│   └── tray.ico
├── go.mod
├── icon_windows.go
├── main_windows.go
└── main_unsupported.go
```
