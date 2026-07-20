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
        # [Console]::Error.WriteLine("WritePacketWorker: Signal received, processing packet queue.")
        $Packet = $null
        while ($PacketQueue.TryDequeue([ref]$Packet)) {
            # [Console]::Error.WriteLine("WritePacketWorker [ch$($Packet.ChannelID),type:$($Packet.Type)]: Packet dequeued. Length: $($Packet.Length)")
            $Header = [BitConverter]::GetBytes($Packet.Type) + 
            [BitConverter]::GetBytes($Packet.ChannelID) + 
            [BitConverter]::GetBytes($Packet.Payload.Length)
            try {
                $OutputStreamWriter.BaseStream.Write($Header, 0, $Header.Length)
                $OutputStreamWriter.BaseStream.Write($Packet.Payload, 0, $Packet.Payload.Length)
                $OutputStreamWriter.Flush()
            }
            catch {
                [Console]::Error.WriteLine("WritePacketWorker [ch$($Packet.ChannelID),type:$($Packet.Type)]: Write error:[$error]")
                continue
            }
            # [Console]::Error.WriteLine("WritePacketWorker [ch$($Packet.ChannelID),type:$($Packet.Type)]: Packet written to output stream.")
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
            # [Console]::Error.WriteLine("PacketWorker [ch:$($Packet.ChannelID) type:$($Packet.TypeNum)]: Response sent.")
        }
        
        [void]StopWorker([Int32]$ChannelID) {
            $this.SendResponse(@{ Type = 2; Payload = [byte[]]::new(0); ChannelID = $ChannelID })
            $null = $this.WorkerInstance.WorkerQueue.Enqueue($this.WorkerInstance)
            # [Console]::Error.WriteLine("PacketWorker [ch:$($ChannelID)]: Worker stopped.")
        }

        [void]Run() {
            # [Console]::Error.WriteLine("PacketWorker started.")
            while ($true) { 
                $null = $this.WorkerInstance.PacketQueueSignal.WaitOne()
                $Packet = $null
                while ($this.WorkerInstance.PacketQueue.TryDequeue([ref]$Packet)) {
                    # [Console]::Error.WriteLine("PacketWorker [ch:$($Packet.channelID) type:$($Packet.TypeNum)]: Packet received.")
                    try {
                        if (!$this.ProcessPacket($Packet)) {
                            # [Console]::Error.WriteLine("PacketWorker [ch:$($Packet.channelID) type:$($Packet.TypeNum)]: Processing failed. Worker will stop.")
                            $this.StopWorker($Packet.ChannelID)
                            continue 
                        }
                    }
                    catch {
                        [Console]::Error.WriteLine("PacketWorker [ch:$($Packet.channelID) type:$($Packet.TypeNum)]: Exception occurred while processing. Error: $($_.Exception.Message). Worker will stop.")
                        $this.NamedPipeStream.Close()
                        $this.NamedPipeStream = $null
                        $this.StopWorker($Packet.ChannelID)
                        Start-Sleep -Seconds 1.0
                        continue
                    }
                }
            }
        }


        [bool]ProcessPacket([hashtable]$Packet) {
            # [Console]::Error.WriteLine("PacketWorker [ch:$($Packet.channelID) type:$($Packet.TypeNum)]: Processing packet.")
            if (0 -eq $Packet.TypeNum) {
                if ($null -ne $this.NamedPipeStream) {
                    # [Console]::Error.WriteLine("PacketWorker [ch:$($Packet.channelID) type:$($Packet.TypeNum)]: Named pipe connection already closed.")
                    $this.NamedPipeStream.Close()
                    $this.NamedPipeStream = $null
                    return $false
                }
                # [Console]::Error.WriteLine("PacketWorker [ch:$($Packet.channelID) type:$($Packet.TypeNum)]: open Named-pipe:openssh-ssh-agent...")
                $this.NamedPipeStream = [System.IO.Pipes.NamedPipeClientStream]::new(".", "openssh-ssh-agent", [System.IO.Pipes.PipeDirection]::InOut)
                $this.NamedPipeStream.Connect()
                $this.WorkerInstance.ChannelID = $Packet.ChannelID
                # [Console]::Error.WriteLine("PacketWorker [ch:$($Packet.channelID) type:$($Packet.TypeNum)]: Named pipe connection established.")
            }
            elseif (2 -eq $Packet.TypeNum) {
                if ($null -eq $this.NamedPipeStream) {
                    # [Console]::Error.WriteLine("PacketWorker [ch:$($Packet.channelID) type:$($Packet.TypeNum)]: No active named pipe connection to close.")
                    return $false
                }
                $this.NamedPipeStream.Close()
                $this.NamedPipeStream = $null
                # [Console]::Error.WriteLine("PacketWorker [ch:$($Packet.channelID) type:$($Packet.TypeNum)]: Named pipe connection closed.")
                return $false
            }

            # Write -> Read

            # [Console]::Error.WriteLine("PacketWorker [ch:$($Packet.channelID) type:$($Packet.TypeNum)]: Named pipe write...")
            $Header = [BitConverter]::GetBytes($Packet.Payload.Length)
            $null = [Array]::Reverse($Header)
            $this.NamedPipeStream.Write($Header, 0, $Header.Length)
            $this.NamedPipeStream.Write($Packet.Payload, 0, $Packet.Payload.Length)
            $this.NamedPipeStream.Flush()
            # [Console]::Error.WriteLine("PacketWorker [ch:$($Packet.channelID) type:$($Packet.TypeNum)]: Data written to named pipe. length:$($Packet.Payload.Length)")
            
            $Header = [byte[]]::new(4)
            $n = $this.NamedPipeStream.Read($Header, 0, $Header.Length)
            if ($n -eq 0) {
                [Console]::Error.WriteLine("PacketWorker [ch:$($Packet.channelID) type:$($Packet.TypeNum)]: header reaad error length:0")
                return $false
            }
            $null = [Array]::Reverse($Header)
            $length = [BitConverter]::ToInt32($Header, 0)
            $Payload = [byte[]]::new($length)
            $n = $this.NamedPipeStream.Read($Payload, 0, $length)
            if ($n -gt 0) {
                # [Console]::Error.WriteLine("PacketWorker [ch:$($Packet.channelID) type:$($Packet.TypeNum)]: read payload:length:$($Payload.Length), n:$($n)")
                $Payload = $Payload[0..($n - 1)]
                # [Console]::Error.WriteLine("PacketWorker [ch:$($Packet.channelID) type:$($Packet.TypeNum)]: read payload:length:$($Payload.Length), n:$($n)")
                $this.SendResponse(@{ Type = 1; Payload = $Payload; ChannelID = $Packet.ChannelID })
                # [Console]::Error.WriteLine("PacketWorker [ch:$($Packet.channelID) type:$($Packet.TypeNum)]: Response read from named pipe and sent.")
                return $true
            }
            return $false
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
        # [Console]::Error.WriteLine("PacketReader: Reading packet from input stream.")
        $Header = [byte[]]::new(12)
        $n = $InputStreamReader.Read($Header, 0, $Header.Length)
        if ($n -eq 0) {
            return @{Error = "PacketReader: Failed to read header (length zero)." }
        }
        $Res = @{
            TypeNum   = [BitConverter]::ToInt32($Header, 0)
            ChannelID = [BitConverter]::ToInt32($Header, 4)
            Length    = [BitConverter]::ToInt32($Header, 8)
            Error     = $null
        }
        # [Console]::Error.WriteLine("PacketReader [ch:$($Res.ChannelID) type:$($Res.TypeNum)]: Header read successfully. Length: $n.")
        $Res.Payload = [byte[]]::new($Res.Length)
        $n = $InputStreamReader.Read($Res.Payload, 0, $Res.Length)
        if ($n -ne $Res.Length) {
            $Res = @{Error = "PacketReader [ch:$($Res.ChannelID) type:$($Res.TypeNum)]: Incomplete payload read. Expected: $($Res.Length), Actual: $n." }
        }
        # [Console]::Error.WriteLine("PacketReader [ch:$($Res.ChannelID) type:$($Res.TypeNum)]: Packet read completed. Length: $($Res.Length).")
        return $Res
    }

    [void] Run() {
        [Console]::Error.WriteLine("PacketReader started.")
        while ($true) {
            # [Console]::Error.WriteLine("PacketReader: Waiting for packets.")
            $Packet = $null
            try {
                $Packet = $this.ReadPacket($this.InputStreamReader)
            }
            catch {
                [Console]::Error.WriteLine("InputStreamRead error: [$error]")
                return
            }
            if ($null -ne $Packet.Error) {
                [Console]::Error.WriteLine($Packet.Error)
                Start-Sleep -Seconds 1.0
                continue
            }
            # [Console]::Error.WriteLine("PacketReader [ch:$($Packet.ChannelID) type:$($Packet.TypeNum)]: Packet received. Length: $($Packet.Length).")
            $WorkerInstance = $null
            if ($this.Channels.ContainsKey($Packet.ChannelID)) {
                $WorkerInstance = $this.Channels[$Packet.ChannelID]
            }
            else {
                if ($this.WorkerQueue.TryDequeue([ref]$WorkerInstance)) {
                    $this.Channels.Remove($WorkerInstance.ChannelID)
                    $WorkerInstance.ChannelID = $Packet.ChannelID
                    # [Console]::Error.WriteLine("PacketReader [ch:$($Packet.ChannelID) type:$($Packet.TypeNum)]: Reusing existing worker.")
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
                    # [Console]::Error.WriteLine("PacketReader [ch:$($Packet.ChannelID) type:$($Packet.TypeNum)]: New worker initialized.")
                }
                $this.Channels[$WorkerInstance.ChannelID] = $WorkerInstance
            }
            $WorkerInstance.PacketQueue.Enqueue($Packet)
            $WorkerInstance.PacketQueueSignal.Set()
            # [Console]::Error.WriteLine("PacketReader [ch:$($Packet.ChannelID) type:$($Packet.TypeNum)]: Packet dispatched to worker.")
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

    $null = [powershell]::Create().AddScript($WritePacketWorker).
    AddArgument($MainPacketQueue).AddArgument($MainPacketQueueSignal).AddArgument($OutputStreamWriter).
    BeginInvoke()
    [Console]::Error.WriteLine("Main: WritePacketWorker started.")

    [PacketReader]::new( $MainPacketQueue, $MainPacketQueueSignal, $InputStreamReader, $PacketWorkerScript).Run()
    $InputStreamReader.Close()
    $OpenStandardOutput.Close()
}

Main
