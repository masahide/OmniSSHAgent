[Console]::OutputEncoding = [System.Text.Encoding]::GetEncoding('utf-8')
#Set-PSDebug -trace 2

$WritePacketWorker = {
    param (
        [System.Collections.Concurrent.ConcurrentQueue[Hashtable]] $PacketQueue,
        [System.Threading.AutoResetEvent] $MainPacketQueueSignal,
        [System.IO.StreamWriter] $OutputStreamWriter
    )

    [Console]::Error.WriteLine("WritePacketWorker started.")
    while ($true) {
        $null = $MainPacketQueueSignal.WaitOne()
        [Console]::Error.WriteLine("WritePacketWorker: Signal received, processing packet queue.")
        $Packet = $null
        if ($PacketQueue.TryDequeue([ref]$Packet)) {
            [Console]::Error.WriteLine("WritePacketWorker: Packet dequeued. Length: $($Packet.Length), Channel ID: $($Packet.ChannelID), Type: $($Packet.Type).")
            $Header = [BitConverter]::GetBytes($Packet.Type) + 
            [BitConverter]::GetBytes($Packet.ChannelID) + 
            [BitConverter]::GetBytes($Packet.Payload.Length)
            $OutputStreamWriter.BaseStream.Write($Header, 0, $Header.Length)
            $OutputStreamWriter.BaseStream.Write($Packet.Payload, 0, $Packet.Payload.Length)
            $OutputStreamWriter.Flush()
            [Console]::Error.WriteLine("WritePacketWorker: Packet written to output stream.")
        }
    }
}

$PacketWorkerScript = {
    param (
        [Hashtable] $WorkerInstance
    )

    class PacketWorker {
        [Hashtable] $WorkerInstance
        [System.IO.Pipes.NamedPipeClientStream] $NamedPipeStream

        PacketWorker([Hashtable] $WorkerInstance) {
            $this.WorkerInstance = $WorkerInstance
            $this.NamedPipeStream = $null
        }

        [void]SendResponse([hashtable]$Packet) {
            $null = $this.WorkerInstance.MainPacketQueue.Enqueue($Packet)
            $null = $this.WorkerInstance.MainPacketQueueSignal.Set()
            [Console]::Error.WriteLine("PacketWorker: Response sent for Channel ID: $($Packet.ChannelID).")
        }
        
        [void]StopWorker([Int32]$ChannelID) {
            $this.SendResponse(@{ Type = 2; Payload = [byte[]]::new(0); ChannelID = $ChannelID })
            $null = $this.WorkerInstance.WorkerQueue.Enqueue($this.WorkerInstance)
            [Console]::Error.WriteLine("PacketWorker: Worker stopped for Channel ID: $ChannelID.")
        }

        [void]Run() {
            [Console]::Error.WriteLine("PacketWorker started.")
            while ($true) { 
                $null = $this.WorkerInstance.PacketQueueSignal.WaitOne()
                $Packet = $null
                if ($this.WorkerInstance.PacketQueue.TryDequeue([ref]$Packet)) {
                    [Console]::Error.WriteLine("PacketWorker: Packet received. Channel ID: $($Packet.ChannelID).")
                    try {
                        if (!$this.ProcessPacket($Packet)) {
                            [Console]::Error.WriteLine("PacketWorker: Processing failed for Channel ID: $($Packet.ChannelID). Worker will stop.")
                            $this.StopWorker($Packet.ChannelID)
                            continue 
                        }
                    }
                    catch {
                        [Console]::Error.WriteLine("PacketWorker: Exception occurred while processing Channel ID: $($Packet.ChannelID). Error: $($_.Exception.Message). Worker will stop.")
                        $this.StopWorker($Packet.ChannelID)
                        continue
                    }
                }
            }
        }

        [bool]ProcessPacket([hashtable]$Packet) {
            [Console]::Error.WriteLine("PacketWorker: Processing packet. Type: $($Packet.TypeNum), Channel ID: $($Packet.ChannelID).")
            if (0 -eq $Packet.TypeNum) {
                if ($null -ne $this.NamedPipeStream) {
                    [Console]::Error.WriteLine("PacketWorker: Named pipe connection already closed. Channel ID: $($Packet.ChannelID).")
                    $this.NamedPipeStream.Close()
                    $this.NamedPipeStream = $null
                    return $false
                }
                $this.NamedPipeStream = [System.IO.Pipes.NamedPipeClientStream]::new(".", "openssh-ssh-agent", [System.IO.Pipes.PipeDirection]::InOut)
                $this.NamedPipeStream.Connect()
                $this.WorkerInstance.ChannelID = $Packet.ChannelID
                [Console]::Error.WriteLine("PacketWorker: Named pipe connection established. Channel ID: $($Packet.ChannelID).")
            }
            elseif (2 -eq $Packet.TypeNum) {
                if ($null -eq $this.NamedPipeStream) {
                    [Console]::Error.WriteLine("PacketWorker: No active named pipe connection to close. Channel ID: $($Packet.ChannelID).")
                    return $false
                }
                $this.NamedPipeStream.Close()
                $this.NamedPipeStream = $null
                [Console]::Error.WriteLine("PacketWorker: Named pipe connection closed. Channel ID: $($Packet.ChannelID).")
                return $false
            }
            $this.NamedPipeStream.Write($Packet.Payload, 0, $Packet.Payload.Length)
            $this.NamedPipeStream.Flush()
            [Console]::Error.WriteLine("PacketWorker: Data written to named pipe. Channel ID: $($Packet.ChannelID).")
            
            $Payload = [byte[]]::new(10240)
            $n = $this.NamedPipeStream.Read($Payload, 0, $Payload.Length)
            if ($n -gt 0) {
                $Payload = $Payload[0..($n - 1)]
                $this.SendResponse(@{ Type = 1; Payload = $Payload; ChannelID = $Packet.ChannelID })
                [Console]::Error.WriteLine("PacketWorker: Response read from named pipe and sent. Channel ID: $($Packet.ChannelID).")
            }
            return $true
        }
    }

    [PacketWorker]::new($WorkerInstance).Run()
}

class PacketReader {
    [System.Collections.Concurrent.ConcurrentQueue[Hashtable]] $MainPacketQueue
    [System.Threading.AutoResetEvent] $MainPacketQueueSignal
    [System.IO.Stream] $InputStreamReader
    [System.Collections.Generic.Dictionary[int, Hashtable]] $Channels
    [System.Collections.Concurrent.ConcurrentQueue[Hashtable]] $WorkerQueue
    [ScriptBlock]$PacketWorkerScript

    PacketReader([System.Collections.Concurrent.ConcurrentQueue[Hashtable]] $MainPacketQueue,
        [System.Threading.AutoResetEvent] $MainPacketQueueSignal,
        [System.IO.Stream] $InputStreamReader, 
        [ScriptBlock] $PacketWorkerScript) {
        $this.MainPacketQueue = $MainPacketQueue
        $this.MainPacketQueueSignal = $MainPacketQueueSignal
        $this.InputStreamReader = $InputStreamReader
        $this.PacketWorkerScript = $PacketWorkerScript
        $this.Channels = [System.Collections.Generic.Dictionary[int, Hashtable]]::new()
        $this.WorkerQueue = [System.Collections.Concurrent.ConcurrentQueue[Hashtable]]::new()
        [Console]::Error.WriteLine("PacketReader initialized.")
    }
    
    [Hashtable] ReadPacket ([System.IO.Stream] $InputStreamReader) {
        [Console]::Error.WriteLine("PacketReader: Reading packet from input stream.")
        $Header = [byte[]]::new(12)
        $n = $InputStreamReader.Read($Header, 0, $Header.Length)
        if ($n -eq 0) {
            return @{Error = "PacketReader: Failed to read header (length zero)." }
        }
        [Console]::Error.WriteLine("PacketReader: Header read successfully. Length: $n.")
        $Res = @{
            TypeNum   = [BitConverter]::ToInt32($Header, 0)
            ChannelID = [BitConverter]::ToInt32($Header, 4)
            Length    = [BitConverter]::ToInt32($Header, 8)
            Error     = $null
        }
        $Res.Payload = [byte[]]::new($Res.Length)
        $n = $InputStreamReader.Read($Res.Payload, 0, $Res.Length)
        if ($n -ne $Res.Length) {
            $Res = @{Error = "PacketReader: Incomplete payload read. Expected: $($Res.Length), Actual: $n. Channel ID: $($Res.ChannelID), Type: $($Res.TypeNum)." }
        }
        [Console]::Error.WriteLine("PacketReader: Packet read completed. Channel ID: $($Res.ChannelID), Type: $($Res.TypeNum), Length: $($Res.Length).")
        return $Res
    }

    [void] Run() {
        [Console]::Error.WriteLine("PacketReader started.")
        while ($true) {
            [Console]::Error.WriteLine("PacketReader: Waiting for packets.")
            $Packet = $this.ReadPacket($this.InputStreamReader)
            if ($null -ne $Packet.Error) {
                [Console]::Error.WriteLine($Packet.Error)
                continue
            }
            [Console]::Error.WriteLine("PacketReader: Packet received. Type: $($Packet.TypeNum), Channel ID: $($Packet.ChannelID), Length: $($Packet.Length).")
            $WorkerInstance = $null
            if ($this.Channels.ContainsKey($Packet.ChannelID)) {
                $WorkerInstance = $this.Channels[$Packet.ChannelID]
            }
            else {
                if ($this.WorkerQueue.TryDequeue([ref]$WorkerInstance)) {
                    $this.Channels.Remove($WorkerInstance.ChannelID)
                    $WorkerInstance.ChannelID = $Packet.ChannelID
                    [Console]::Error.WriteLine("PacketReader: Reusing existing worker for Channel ID: $($Packet.ChannelID).")
                }
                else {
                    $WorkerInstance = @{
                        MainPacketQueue       = $this.MainPacketQueue
                        MainPacketQueueSignal = $this.MainPacketQueueSignal
                        WorkerQueue           = $this.WorkerQueue
                        ChannelID             = $Packet.ChannelID
                        PacketQueue           = [System.Collections.Concurrent.ConcurrentQueue[Hashtable]]::new()
                        PacketQueueSignal     = [System.Threading.AutoResetEvent]::new($false)
                    }
                    $null = [PowerShell]::Create().AddScript($this.PacketWorkerScript).
                    AddArgument($WorkerInstance).BeginInvoke()
                    [Console]::Error.WriteLine("PacketReader: New worker initialized for Channel ID: $($Packet.ChannelID).")
                }
                $this.Channels[$WorkerInstance.ChannelID] = $WorkerInstance
            }
            $WorkerInstance.PacketQueue.Enqueue($Packet)
            $WorkerInstance.PacketQueueSignal.Set()
            [Console]::Error.WriteLine("PacketReader: Packet dispatched to worker. Channel ID: $($Packet.ChannelID).")
        }
    }
}

function Main {
    $OutputStreamWriter = [Console]::OpenStandardOutput()
    $m = [system.Text.Encoding]::UTF8.GetBytes("startAgent")
    $OutputStreamWriter.Write($m, 0, $m.Length)
    $OutputStreamWriter.Flush()
    [Console]::Error.WriteLine("Main: Agent start message sent to output stream.")

    $MainPacketQueue = [System.Collections.Concurrent.ConcurrentQueue[Hashtable]]::new()
    $MainPacketQueueSignal = [System.Threading.AutoResetEvent]::new($false)
    $InputStreamReader = [Console]::OpenStandardInput()
    [Console]::Error.WriteLine("Main: Initialized input/output streams.")

    $null = [powershell]::Create().AddScript($WritePacketWorker).
    AddArgument($MainPacketQueue).AddArgument($MainPacketQueueSignal).AddArgument($OutputStreamWriter).
    BeginInvoke()
    [Console]::Error.WriteLine("Main: WritePacketWorker started.")

    [PacketReader]::new( $MainPacketQueue, $MainPacketQueueSignal, $InputStreamReader, $PacketWorkerScript).Run()
    $InputStreamReader.Close()
    $OpenStandardOutput.Close()
}

Main
