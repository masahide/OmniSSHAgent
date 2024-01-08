<script>
  import Paper, { Content } from "@smui/paper";
  import { Icon } from "@smui/icon-button";
  import Button, { Label } from "@smui/button";
  import AddFileDialog from "./AddFileDialog.svelte";
  import { toast } from "@zerodevx/svelte-toast";
  import Accordion, { Panel, Header } from "@smui-extra/accordion";
  import Textfield from "@smui/textfield";
  import { onMount } from "svelte";

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

  const onLoadKeysEvent = (message) => {
    loadKeys();
    console.log(message);
  };

  let data = { ProxyModeOfNamedPipe: false };
  onMount(async () => {
    await window.go.main.App.GetSettings()
      .then((savedata) => {
        console.log(savedata);
        data = { ...savedata };
      })
      .catch((err) => {
        console.error(err);
        toast.push(err, red);
      });
  });

  window.runtime.EventsOn("LoadKeysEvent", onLoadKeysEvent);

  function handleData(event) {
    //window.runtime.LogDebug(JSON.stringify(event.detail));
    addlocalFile(event.detail);
  }
  const delKey = async (sha256) => {
    await window.go.main.App.DeleteKey(sha256)
      .then(() => {
        toast.push("Successfully deleted key", green);
        loadKeys();
      })
      .catch((err) => {
        if (err == "cancel") {
          return;
        }
        console.error("failed to delete key with error:" + err);
        toast.push(err, red);
      });
  };
  const addlocalFile = async (privateKeyFile) => {
    await window.go.main.App.AddLocalFile(privateKeyFile)
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
        if (!data.ProxyModeOfNamedPipe) {
          toast.push(err, red);
        }
      });
  };
  function getKeyTitle(key) {
    if (key.storeType == "") {
      return "---";
    }
    //return key.name;
    return key.name;
  }
  loadKeys();
</script>

<div class="accordion-container">
  <Accordion class="keylist">
    {#each keys as key, i}
      <Panel>
        <Header
          >{getKeyTitle(key)}<span slot="description"
            ><div class="sha256">{key.publickey.sha256}</div></span
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
          <Paper variant="outlined" class="publickey"
            ><Content>{key.publickey.string}</Content></Paper
          >
          {#if !data.ProxyModeOfNamedPipe}
            <Button variant="outlined" on:click={delKey(key.publickey.sha256)}
              ><Icon class="material-icons">delete</Icon><Label>Delete</Label
              ></Button
            >
          {/if}
        </Content>
      </Panel>
    {/each}
  </Accordion>
</div>

{#if !data.ProxyModeOfNamedPipe}
  <AddFileDialog on:eventAddPkfile={handleData} />
{/if}
<svelte:body on:mouseenter={loadKeys} on:mouseleave={loadKeys} />

<style>
  .accordion-container {
    overflow-wrap: break-word;
  }

  * :global(.keylist .smui-accordion__header__title--with-description) {
    flex-basis: 20% !important;
    max-width: 200px !important;
  }
</style>
