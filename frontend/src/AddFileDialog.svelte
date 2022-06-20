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
        duration: 7000, // duration of progress bar tween to the `next` value
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

    const newPkfile = () => {
        return {
            filePath: "",
            type: "",
            encryption: false,
            passphrase: "",
            publickey: {
                type: "",
            },
        };
    };
    let pkFile = newPkfile();

    $: keytype = pkFile.fileType + ":" + pkFile.publickey.type

    const dispatch = createEventDispatcher();
    function add() {
        dispatch("eventAddPkfile", pkFile );
        open = false;
        addButton = false;
        pkFile = newPkfile();
    }
    const openFile = async () => {
        await window.go.main.App.OpenFile()
            .then((file) => {
                pkFile.filePath = file;
                pkFile.passphrase = "";
                checkKeyType();
            })
            .catch((err) => {
                console.error("OpenFile error:" + err);
                addButton = false;
                toast.push(err, red);
            });
    };
    const checkKeyType = async () => {
        await window.go.main.App.CheckKeyType(pkFile.filePath, pkFile.passphrase)
            .then((file) => {
                let pass = pkFile.passphrase;
                pkFile = { ...file };
                pkFile.passphrase = pass;
                console.debug(pkFile);
                addButton = true;
                if (pkFile.encryption && pkFile.passphrase.length > 0) {
                    toast.push("Decrypted secreet key", green);
                }
                if (pkFile.encryption && pkFile.passphrase.length == 0) {
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
        <Title id="mandatory-title">Add a Private key</Title>
        <Content id="mandatory-content">
            <Card padded>
                <div>
                    <div>
                        <FormField style="width: 100%;">
                            <Textfield
                                disabled
                                value={pkFile.filePath}
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
                                bind:checked={pkFile.encryption}
                                disabled
                                value="Encryption?"
                            />
                            <span
                                >{pkFile.encryption
                                    ? "Encrypted with passphrase"
                                    : "Not encrypted"}</span
                            >
                        </FormField>
                    </div>
                    {#if pkFile.encryption}
                        <FormField style="width: 100%;">
                            <Textfield
                                bind:value={pkFile.passphrase}
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
    <Label>Open new file</Label>
</Button>

<style>
    .dialog {
        margin-left: 8px;
        margin-right: 8px;
    }
</style>
