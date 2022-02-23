<script>
    import { createEventDispatcher } from "svelte";
    import Paper, { Title, Subtitle, Content } from "@smui/paper";
    import Dialog, { Actions } from "@smui/dialog";
    import Button, { Label } from "@smui/button";
    import List, { Item, Text, PrimaryText, SecondaryText } from "@smui/list";
    import Textfield from "@smui/textfield";
    import HelperText from "@smui/textfield/helper-text";
    import Card from "@smui/card";
    import FormField from "@smui/form-field";
    import Switch from "@smui/switch";
    import { toast } from "@zerodevx/svelte-toast";

    let open = false;
    let addButton = false;

    const red = {
        theme: {
            "--toastBackground": "#F56565",
            "--toastBarBackground": "#C53030",
        },
    };
    const green = {
        theme: {
            "--toastBackground": "#48BB78",
            "--toastBarBackground": "#2F855A",
        },
    };

    const newData = () => {
        return {
            filePath: "",
            type: "",
            encryption: false,
            passphrase: "",
        };
    };
    let data = newData();

    $: keytype = data.type + ":" + data.algo

    const dispatch = createEventDispatcher();
    function add() {
        dispatch("data", { data: data });
        open = false;
        addButton = false;
        data = newData();
    }
    const openFile = async () => {
        await window.go.main.App.OpenFile()
            .then((file) => {
                data.filePath = file;
                data.passphrase = "";
                checkKeyType();
            })
            .catch((err) => {
                console.error("OpenFile error:" + err);
                addButton = false;
                toast.push(err, red);
            });
    };
    const checkKeyType = async () => {
        await window.go.main.App.CheckKeyType(data.filePath, data.passphrase)
            .then((file) => {
                let pass = data.passphrase;
                data = { ...file };
                data.passphrase = pass;
                console.debug(data);
                addButton = true;
                if (data.encryption && data.passphrase.length > 0) {
                    toast.push("Successful decryption", green);
                }
                if (data.encryption && data.passphrase.length == 0) {
                    addButton = false;
                }
            })
            .catch((err) => {
                console.error("checkKeyTpye error:" + err);
                addButton = false;
                toast.push(err, red);
            });
    };
</script>

<Dialog
    bind:open
    scrimClickAction=""
    escapeKeyAction=""
    surface$style="width: 850px; max-width: calc(100vw - 32px);"
    aria-labelledby="mandatory-title"
    aria-describedby="mandatory-content"
>
    <div class="dialog">
        <Title id="mandatory-title">Add Private key</Title>
        <Content id="mandatory-content">
            <Card padded>
                <div>
                    <div>
                        <FormField style="width: 100%;">
                            <Textfield
                                disabled
                                value={data.filePath}
                                label="Private key file"
                                style="width: 100%;"
                                helperLine$style="width: 100%;"
                            >
                                <HelperText slot=".ppk, id_rsa..." />
                            </Textfield>
                        </FormField>
                    </div>
                    <div>
                        <Button on:click={openFile} variant="raised">
                            <Label>Open file</Label>
                        </Button>
                    </div>
                    <div>
                        <FormField style="width: 100%;">
                            <Textfield
                                disabled
                                value={keytype}
                                label="key type"
                                style="width: 100%;"
                                helperLine$style="width: 100%;"
                            >
                                <HelperText slot="private key type" />
                            </Textfield>
                        </FormField>
                    </div>
                    <div>
                        <FormField>
                            <Switch
                                bind:checked={data.encryption}
                                disabled
                                value="Encryption?"
                            />
                            <span
                                >{data.encryption
                                    ? "Encrypt with passphrase"
                                    : "Not encrypted"}</span
                            >
                        </FormField>
                    </div>
                    {#if data.encryption}
                        <FormField style="width: 100%;">
                            <Textfield
                                bind:value={data.passphrase}
                                type="password"
                                label="passphrase"
                                style="width: 100%;"
                                helperLine$style="width: 100%;"
                            >
                                <HelperText slot="passphrase of private key" />
                            </Textfield>
                        </FormField>
                        <Button on:click={checkKeyType}>
                            <Label>check</Label>
                        </Button>
                    {/if}
                </div>
            </Card>
        </Content>
        <Actions>
            {#if addButton}
                <Button on:click={add}>
                    <Label>Add</Label>
                </Button>
            {/if}
            <Button on:click={() => (open = false)}>
                <Label>Cancel</Label>
            </Button>
        </Actions>
    </div>
</Dialog>

<Button on:click={() => (open = true)}>
    <Label>New open file</Label>
</Button>

<style>
    .dialog {
        margin-left: 8px;
        margin-right: 8px;
    }
</style>
