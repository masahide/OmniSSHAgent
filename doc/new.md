## 結論

その構成が今回の再構築では最も適しています。

特に重要なのは、添付コード内にすでに近い実装が存在している点です。

* `cmd/omni-socat`
* `pkg/npipe2stdin`
* `hack/ubuntu.setup.sh`

この旧構成は、WSL側のUnixソケットを`Socat`で受け、接続ごとにWindows用Goバイナリを起動し、標準入出力とNamed Pipeを接続しています。

つまり今回やるべきことは、まったく新しい方式を考案するというより、次のように整理できます。

1. Windows側の`omni-socat.exe`相当を整理して再利用
2. `Socat`をLinux用Goプロセスへ置換
3. PowerShellで実装された多重化ワーカーを廃止
4. PowerShellインストーラーでWindows側バイナリを正規に配置

## 推奨するWSLブリッジ構成

最初の実装では、接続ごとにWindowsブリッジを起動する方式を推奨します。

```text
WSL内のSSHクライアント
        │
        │ SSH_AUTH_SOCK
        ▼
Unix Domain Socket
        │
        ▼
omnisshagent-wsl-proxy
Linux用Goプロセス
        │
        │ 接続ごとに起動
        ▼
omnisshagent-bridge.exe
Windows用Goプロセス
        │
        ▼
Windows Named Pipe
        │
        ▼
OmniSSHAgent
```

WSLはLinux側からWindowsの実行ファイルを直接実行でき、パイプ、リダイレクト、バックグラウンド実行も通常のLinuxプロセスに近い形で扱えます。したがって、標準入力と標準出力をバイナリストリームとして使う設計はWSLの公式な相互運用モデルに沿っています。([Microsoft Learn][1])

### Linux側の処理

Linux側は次の処理だけを担当します。

```text
1. Unix Domain Socketをlisten
2. 接続をaccept
3. omnisshagent-bridge.exeを起動
4. Unix Socketから子プロセスのstdinへコピー
5. 子プロセスのstdoutからUnix Socketへコピー
6. 接続終了時に子プロセスを終了
```

概念的には次の形です。

```go
func handleConnection(ctx context.Context, conn net.Conn, bridgePath string) error {
	defer conn.Close()

	cmd := exec.CommandContext(ctx, bridgePath)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	errCh := make(chan error, 2)

	go func() {
		_, err := io.Copy(stdin, conn)
		_ = stdin.Close()
		errCh <- err
	}()

	go func() {
		_, err := io.Copy(conn, stdout)
		errCh <- err
	}()

	<-errCh

	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}

	return cmd.Wait()
}
```

実装時は、コピーの片側が正常なEOFで終了したケースと、異常終了したケースを分ける必要があります。また、ゴルーチンリークを防ぐため、接続、stdin、プロセスを確実に閉じる終了制御が必要です。

### Windows側の処理

Windows側は非常に単純です。

```text
stdin  → Named Pipe
stdout ← Named Pipe
```

既存の`pkg/npipe2stdin`がほぼこの役割を実装しています。

```go
conn, err := winio.DialPipe(pipePath, timeout)
if err != nil {
	return err
}
defer conn.Close()

errCh := make(chan error, 2)

go func() {
	_, err := io.Copy(conn, os.Stdin)
	errCh <- err
}()

go func() {
	_, err := io.Copy(os.Stdout, conn)
	errCh <- err
}()

return <-errCh
```

重要なのは、標準出力にはSSH Agentのバイナリデータ以外を一切出さないことです。

ログ、デバッグ情報、エラーはすべて標準エラーへ出します。

## 最初から多重化しない理由

現在の`cmd/wsl2-ssh-agent-proxy`は、一つのPowerShellプロセス上で複数のSSH Agent接続を多重化しています。

しかし、添付コードを確認すると、現在の多重化処理には改善すべき箇所があります。

* プロトコルのエンディアンが実行環境依存
* PowerShell側のStream Readが部分読み込みを考慮していない
* Linux側のpayload長に上限がない箇所がある
* チャネル終了と送信の競合が起こり得る
* 標準入力への複数書き込みの同期が不明確
* PowerShellプロセスの再起動とUnixソケット接続のライフサイクルが複雑
* SSH Agentプロトコルを一度分解して再構成している

Unixソケット、標準入出力、Named Pipeはいずれもストリームとして扱えるため、接続を一対一で対応させれば、独自の多重化プロトコルは不要です。

```text
Unix接続1 ─ Windowsブリッジ1 ─ Named Pipe接続1
Unix接続2 ─ Windowsブリッジ2 ─ Named Pipe接続2
Unix接続3 ─ Windowsブリッジ3 ─ Named Pipe接続3
```

これにより、SSH Agentプロトコルの4バイト長ヘッダーをLinux側で解釈する必要もなくなります。単純にバイト列を双方向コピーできます。

### プロセス起動コストについて

唯一の欠点は、SSH Agent接続ごとにWindowsプロセスを起動することです。

ただし、SSH認証処理では通常、ネットワーク接続やSSHハンドシェイクの時間も発生します。まず単純な方式で実装し、既存の`cmd/agent-bench`を拡張して測定する方がよいです。

次を測定します。

* `ssh-add -l`相当の初回応答時間
* 連続100回の平均値
* p50、p95、p99
* 16接続、32接続の並列実行
* Git操作時の体感差
* Windowsブリッジプロセスの最大同時数

実測で問題になった場合だけ、Go対Goの常駐多重化方式へ移行します。

## 常駐多重化へ移行する場合

将来的に一つのWindowsブリッジを常駐させる場合は、現在のプロトコルをそのまま移植するのではなく、プロトコルを明文化した方がよいです。

```text
Magic        4 bytes
Version      1 byte
Type         1 byte
Reserved     2 bytes
Channel ID   4 bytes
Length       4 bytes
Payload      N bytes
```

パケット種別は次の程度で十分です。

```text
OPEN
DATA
CLOSE
ERROR
PING
PONG
```

実装条件は次のとおりです。

* エンディアンは固定
* `io.ReadFull`を使用
* 最大フレームサイズを設定
* WriterをMutexで直列化
* チャネルごとに上限付きキューを使用
* チャネルIDの再利用規則を定義
* プロトコルバージョンを交換
* 子プロセス再起動時に全チャネルを終了
* 標準出力をプロトコル専用にする

ただし、これは第二段階で十分です。

## Named Pipeの構成も整理する

WSLブリッジが接続するNamed Pipeは固定した方がよいです。

```text
\\.\pipe\omnisshagent
```

OmniSSHAgent内部で、実際の接続先を切り替えます。

```text
\\.\pipe\omnisshagent
        │
        ├─ OmniSSHAgent自身の鍵ストア
        │
        └─ Windows OpenSSH Agentへのプロキシ
           \\.\pipe\openssh-ssh-agent
```

こうすると、WSL側はOmniSSHAgentがローカル鍵モードなのか、1PasswordやWindows OpenSSHへのプロキシモードなのかを知る必要がありません。

現在の`omni-socat.exe`はCredential Managerの設定を読み、接続可能か判断していますが、その責務はWindowsのOmniSSHAgent本体へ寄せた方が明確です。

Windowsブリッジは、常に固定Named Pipeへ接続するだけにします。

## インストーラーについて

Claude Codeと同様に、利用者向けには次の形が適切です。

```powershell
irm https://github.com/masahide/OmniSSHAgent/releases/latest/download/install.ps1 | iex
```

Claude CodeもWindows向けネイティブインストール方法として、`irm ... | iex`方式を公式に案内しています。([Claude][2])

### 起動ロックに関する整理

この方式によって改善する可能性は高いですが、起動ロックを完全に回避できると断定するのは避けた方がよいです。

Microsoftのドキュメントでは、`Invoke-RestMethod`や`Invoke-WebRequest`などのダウンロード方法は、ファイルへInternet Zoneの印を付けない場合があると説明されています。つまり、ブラウザからZIPをダウンロードして展開する方法より、Mark of the Webの影響を受けにくいという理解は妥当です。([Microsoft Learn][3])

また、`irm | iex`ではインストールスクリプト自体を`.ps1`ファイルとして保存しないため、ダウンロード済みPowerShellスクリプトに対する`RemoteSigned`制限も受けにくくなります。RemoteSignedでは、Internet Zone由来の未署名スクリプトがブロックされ、`Unblock-File`で解除できることがMicrosoftから説明されています。([Microsoft Learn][4])

ただし、次は別の仕組みです。

* Microsoft Defender SmartScreen
* Smart App Control
* AppLocker
* App Control for Business
* ウイルス対策ソフト
* 組織の実行許可ポリシー

SmartScreenはダウンロードされた実行ファイルの発行元とファイルハッシュの評判を評価します。未署名バイナリや新しいバイナリでは警告が残る可能性があります。([Microsoft Learn][5])

したがってインストーラーでは、偶然Mark of the Webが付かないことに依存せず、明示的に処理します。

```powershell
Get-ChildItem $InstallDir -Recurse -File | Unblock-File
```

さらに長期的には、WindowsバイナリをAuthenticode署名するのが望ましいです。

## 推奨するインストール処理

インストール先は管理者権限を要求しないユーザー領域がよいです。

```text
%LOCALAPPDATA%\Programs\OmniSSHAgent
├─ bin
│  ├─ OmniSSHAgent.exe
│  └─ omnisshagent-bridge.exe
├─ uninstall.ps1
└─ version.json
```

インストーラーは次の処理を行います。

1. WindowsのCPUアーキテクチャを判定
2. GitHub ReleaseからZIPを一時ディレクトリへダウンロード
3. SHA-256チェックサムを取得
4. ダウンロードファイルのチェックサムを検証
5. ZIPを展開
6. Authenticode署名がある場合は署名を検証
7. 展開された全ファイルへ`Unblock-File`
8. インストールディレクトリへ原子的に配置
9. `bin`をユーザーPATHへ追加
10. 自動起動を登録
11. `OmniSSHAgent.exe --version`で起動確認
12. 必要に応じてWSLセットアップを実行

次のような引数も用意すると運用しやすくなります。

```powershell
& ([scriptblock]::Create(
    irm https://github.com/masahide/OmniSSHAgent/releases/latest/download/install.ps1
)) -Version stable -InstallWSL
```

想定オプションは次の程度です。

```text
-Version latest
-Version stable
-Version v1.2.3
-InstallWSL
-NoStartup
-InstallDir
-Force
```

`Set-ExecutionPolicy Bypass`を恒久的に設定する実装は避けます。

## WSL側バイナリの配置

Linux側は次の構成が扱いやすいです。

```text
~/.local/bin/omnisshagent-wsl-proxy
~/.config/omnisshagent/config.json
```

ソケットは可能なら次を使います。

```text
$XDG_RUNTIME_DIR/omnisshagent/agent.sock
```

`XDG_RUNTIME_DIR`がない環境ではフォールバックします。

```text
$HOME/.local/run/omnisshagent/agent.sock
```

権限は次のようにします。

```text
ソケットディレクトリ 0700
Unixソケット         0600
```

シェル設定は、複雑なシェルスクリプトではなく次の程度にできます。

```sh
export SSH_AUTH_SOCK="${XDG_RUNTIME_DIR:-$HOME/.local/run}/omnisshagent/agent.sock"
omnisshagent-wsl-proxy ensure >/dev/null 2>&1
```

`ensure`は、プロキシが未起動なら起動し、起動済みなら何もしないコマンドです。

### Windowsブリッジの探索

Linux側は最初にPATHから探索します。

```go
bridgePath, err := exec.LookPath("omnisshagent-bridge.exe")
```

WSLはWindows側PATHをLinux側PATHへ追加する設定を標準で持っています。ただし、WSLの`interop`や`appendWindowsPath`が無効にされている環境では利用できません。([Microsoft Learn][6])

そのため、次も用意します。

```text
--bridge-path /mnt/c/Users/.../omnisshagent-bridge.exe
OMNISSHAGENT_BRIDGE_PATH
```

見つからない場合には、次のような明確なエラーを出します。

```text
omnisshagent-bridge.exe was not found in the WSL PATH.
Restart WSL after installation or specify OMNISSHAGENT_BRIDGE_PATH.
```

## 推奨するコード構成

```text
cmd
├─ omnisshagent
│  └─ main_windows.go
├─ omnisshagent-bridge
│  └─ main_windows.go
└─ omnisshagent-wsl-proxy
   └─ main_linux.go

internal
├─ app
├─ agent
│  ├─ manager.go
│  └─ upstream.go
├─ namedpipe
│  ├─ listener_windows.go
│  └─ client_windows.go
├─ streamproxy
│  └─ copy.go
├─ wslproxy
│  ├─ listener_linux.go
│  ├─ bridge_linux.go
│  └─ lifecycle_linux.go
├─ webui
└─ tray

scripts
├─ install.ps1
├─ uninstall.ps1
└─ install-wsl.sh
```

既存コードとの対応は次のようになります。

| 現在のコード                                | 新しい扱い                        |
| ------------------------------------- | ---------------------------- |
| `pkg/npipe2stdin`                     | Windowsブリッジとして整理して再利用        |
| `cmd/omni-socat`                      | `cmd/omnisshagent-bridge`へ改名 |
| `cmd/wsl2-ssh-agent-proxy`            | LinuxのUnixソケット管理だけに単純化       |
| `pwsh.ps1`                            | 削除                           |
| `Multiplexer`                         | 最初は削除                        |
| `hack/ubuntu.setup.sh`                | Goプロキシのインストール処理へ置換           |
| `hack/ubuntu.wsl2-ssh-agent-proxy.sh` | シェル初期化用の短い設定だけ残す             |
| `socat`依存                             | 削除                           |

## 実装順序

今回の再構築は次の順番が安全です。

### 第一段階

1. Windows側ブリッジを`cmd/omni-socat`から分離
2. 固定Named Pipeへ接続するだけの実装に変更
3. Linux側GoプロキシでUnixソケットをlisten
4. 接続ごとにWindowsブリッジを起動
5. `ssh-add -l`とSSH署名をテスト
6. PowerShellワーカーと多重化処理を削除

### 第二段階

1. PowerShellワンライナーインストーラーを作成
2. チェックサム検証を追加
3. `Unblock-File`を追加
4. ユーザーPATHへ登録
5. WSLセットアップコマンドを追加
6. アップデートとアンインストールを追加

### 第三段階

1. トレイとローカルWeb UIへ移行
2. Wails、Svelte、Node.jsを削除
3. Windows標準ファイル選択ダイアログを実装
4. AgentManagerと設定処理を分離

### 第四段階

接続ごとのWindowsプロセス起動が性能問題になる場合だけ、Go対Goの常駐多重化を実装します。

## 最終的な判断

今回の方針は次の形にすると、非常に整理された構成になります。

```text
Windows
OmniSSHAgent本体
トレイ
ローカルWeb UI
Named Pipe Agent
Windowsブリッジ

WSL
Unix Domain Socketプロキシ

プロセス間
標準入力と標準出力によるバイナリ転送
```

特に、現在のPowerShell実装をGoへ一対一で翻訳する必要はありません。

既存の`npipe2stdin`方式を復活させ、`Socat`だけをLinux用Goプロセスに置き換えるのが、最も小さく、堅牢で、テストしやすい再構築です。インストーラーによってWindowsブリッジを正規配置できるなら、以前この方式を断念した主要な理由も解消できます。

[1]: https://learn.microsoft.com/en-us/windows/wsl/filesystems "Working across file systems | Microsoft Learn"
[2]: https://code.claude.com/docs/en/setup "Advanced setup - Claude Code Docs"
[3]: https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_execution_policies?view=powershell-7.6 "about_Execution_Policies - PowerShell | Microsoft Learn"
[4]: https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.utility/unblock-file?view=powershell-7.6 "Unblock-File (Microsoft.PowerShell.Utility) - PowerShell | Microsoft Learn"
[5]: https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/smartscreen-reputation?utm_source=chatgpt.com "SmartScreen reputation for Windows app developers - Windows apps | Microsoft Learn"
[6]: https://learn.microsoft.com/ja-jp/windows/wsl/wsl-config "WSL での詳細設定の構成 | Microsoft Learn"
