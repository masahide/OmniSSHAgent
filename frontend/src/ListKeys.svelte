<script>
    import Paper, { Content } from "@smui/paper";
    import { Icon } from "@smui/icon-button";
    import Button, { Label } from "@smui/button";
    import AddFileDialog from "./AddFileDialog.svelte";
    import { toast } from "@zerodevx/svelte-toast";
    import Accordion, { Panel, Header } from "@smui-extra/accordion";
    import Textfield from "@smui/textfield";

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

    function handleData(event) {
        window.runtime.LogDebug(JSON.stringify(event.detail));
        addkey(event.detail);
    }
    const delKey = async (sha256) => {
        await window.go.main.App.DeleteKey(sha256)
            .then(() => {
                toast.push("Successful delete key", green);
                loadKeys();
            })
            .catch((err) => {
                if (err=="cancel") {
                    return
                }
                console.error("delete key err:" + err);
                toast.push(err, red);
            });
    };
    const addkey = async (privateKeyFile) => {
        await window.go.main.App.AddKey(privateKeyFile)
            .then(() => {
                toast.push("Successful add key", green);
                loadKeys();
            })
            .catch((err) => {
                console.error("addkey err:" + err);
                toast.push(err, red);
            });
    };
    let keys = [];
    const loadKeys = async () => {
        await window.go.main.App.KeyList()
            .then((list) => {
                keys = [...list];
            })
            .catch((err) => {
                console.error("KeyList err:" + err);
                toast.push(err, red);
            });
    };
    loadKeys();
</script>

<div class="accordion-container">
    <Accordion class="keylist">
        {#each keys as key, i}
            <Panel>
                <Header
                    >{key.filePath.replace(/^.*[\\\/]/, "")}<span
                        slot="description">{key.publickey.sha256}</span
                    ></Header
                >
                <Content
                    ><Textfield
                        disabled
                        style="width: 100%;"
                        label="File path of the private key:"
                        value={key.filePath}
                    /><br /><Textfield
                        disabled
                        label="SSH key type:"
                        value={key.publickey.type}
                    /><br /><Textfield
                        disabled
                        style="width: 100%;"
                        label="Fingerprint SHA256:"
                        value={key.publickey.sha256}
                    /><br /><Textfield
                        disabled
                        style="width: 100%;"
                        label="Fingerprint MD5:"
                        value={key.publickey.md5}
                    /><br />
                    SSH Public key:
                    <Paper variant="outlined"
                        ><Content>{key.publickey.string}</Content></Paper
                    >
                    <Button variant="outlined" on:click={delKey(key.publickey.sha256)} 
                        ><Icon class="material-icons">delete</Icon><Label
                            >Delete</Label
                        ></Button
                    >
                </Content>
            </Panel>
        {/each}
    </Accordion>
</div>

<AddFileDialog on:eventAddPkfile={handleData} />

<style>
    .accordion-container {
        overflow-wrap: break-word;
    }

    * :global(.keylist .smui-accordion__header__title--with-description) {
        flex-basis: 20% !important;
        max-width: 200px !important;
    }
</style>
