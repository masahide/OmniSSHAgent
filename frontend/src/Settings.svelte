<script>
  import { Title, Content } from "@smui/paper";
  import Dialog, { Actions } from "@smui/dialog";
  import Button, { Label } from "@smui/button";
  import Textfield from "@smui/textfield";
  import HelperText from "@smui/textfield/helper-text";
  import Card from "@smui/card";
  import FormField from "@smui/form-field";
  import Switch from "@smui/switch";
  import { toast } from "@zerodevx/svelte-toast";
  import IconButton from "@smui/icon-button";
  import Banner from "@smui/banner";

  let open = false;

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

  let data = {
    StartHidden: false,
    PageantAgent: false,
    NamedPipeAgent: false,
    UnixSocketAgent: false,
    CygWinAgent: false,
    ShowBalloon: false,
    UnixSocketPath: "",
    CygWinSocketPath: "",
    ProxyModeOfNamedPipe: false,
  };

  const openDialog = async () => {
    await window.go.main.App.GetSettings()
      .then((savedata) => {
        console.log(savedata);
        data = { ...savedata };
        open = true;
      })
      .catch((err) => {
        console.error(err);
        toast.push(err, red);
      });
  };
  const save = async () => {
    await window.go.main.App.Save(data)
      .then(() => {
        open = false;
        toast.push("Saved the settings", green);
      })
      .catch((err) => {
        console.error(err);
        toast.push(err, red);
      });
  };
  const namePipeToggle = async (e) => {
    if (data.ProxyModeOfNamedPipe) {
      data.ProxyModeOfNamedPipe = false;
    }
  };
  const proxyToggle = async (e) => {
    if (data.ProxyModeOfNamedPipe) {
      data.NamedPipeAgent = false;
    }
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
  <Banner open fixed mobileStacked content$style="max-width: max-content;">
    <Label slot="label">These settings will take effect after a restart.</Label>
    <Button slot="actions">Understood</Button>
  </Banner>
  <div class="dialog">
    <Title id="mandatory-title">Settings</Title>
    <Content id="mandatory-content">
      <Card padded>
        <div>
          <div>
            <FormField>
              <Switch
                bind:checked={data.StartHidden}
                value="Minimize to system tray on launch?"
              />
              <span
                >{data.StartHidden
                  ? "Hide the window on launch"
                  : "Show window on launch"}</span
              >
            </FormField>
          </div>
          <div>
            <FormField>
              <Switch
                bind:checked={data.ShowBalloon}
                value="Show a balloon notification when an SSH key is used"
              />
              <span
                >{data.ShowBalloon
                  ? "Show a balloon notification when an SSH key is used"
                  : "Do not show a balloon notification when an SSH key is used"}</span
              >
            </FormField>
          </div>
          <div>
            <FormField>
              <Switch bind:checked={data.PageantAgent} value="Enable pageant" />
              <span
                >{data.PageantAgent
                  ? "Enable Pageant"
                  : "Disable pageant"}</span
              >
            </FormField>
          </div>
          <div>
            <FormField>
              <Switch
                bind:checked={data.NamedPipeAgent}
                on:SMUISwitch:change={namePipeToggle}
                value="Enable Named pipe agent"
              />
              <span
                >{data.NamedPipeAgent
                  ? "Enable Named pipe agent"
                  : "Disable Named pipe agent"}</span
              >
            </FormField>
          </div>
          <div>
            <FormField>
              <Switch
                bind:checked={data.UnixSocketAgent}
                value="Enable Unix domain socket agent"
              />
              <span
                >{data.UnixSocketAgent
                  ? "Enable Unix domain socket agent"
                  : "Disable Unix domain socket agent"}</span
              >
            </FormField>
          </div>
          {#if data.UnixSocketAgent}
            <div>
              <FormField style="width: 100%;">
                <Textfield
                  bind:value={data.UnixSocketPath}
                  label="Unix domain socket file path(WSL1):"
                  style="width: 100%;"
                  helperLine$style="width: 100%;"
                >
                  <HelperText slot="Set the path of Unix domain socket file" />
                </Textfield>
              </FormField>
            </div>
          {/if}
          <div>
            <FormField>
              <Switch
                bind:checked={data.CygWinAgent}
                value="Enable Cygwin unix domain socket agent"
              />
              <span
                >{data.CygWinAgent
                  ? "Enable Cygwin unix domain socket agent"
                  : "Disable Cygwin unix domain socket agent"}</span
              >
            </FormField>
          </div>
          {#if data.CygWinAgent}
            <div>
              <FormField style="width: 100%;">
                <Textfield
                  bind:value={data.CygWinSocketPath}
                  label="Cygwin Unix domain socket file path(MSYS2):"
                  style="width: 100%;"
                  helperLine$style="width: 100%;"
                >
                  <HelperText
                    slot="Set the path of Cygwin(MSYS2) Unix domain socket file"
                  />
                </Textfield>
              </FormField>
            </div>
          {/if}
          <div>
            <FormField>
              <Switch
                bind:checked={data.ProxyModeOfNamedPipe}
                on:SMUISwitch:change={proxyToggle}
                value="Enable proxy mode for 1Password key-agent"
              />
              <span
                >{data.ProxyModeOfNamedPipe
                  ? "Enable proxy mode for 1Password key-agent"
                  : "Disable proxy mode for 1Password key-agent"}</span
              >
            </FormField>
          </div>
        </div>
      </Card>
    </Content>
    <Actions>
      <Button on:click={save}>
        <Label>OK</Label>
      </Button>
      <Button on:click={() => (open = false)}>
        <Label>Cancel</Label>
      </Button>
    </Actions>
  </div>
</Dialog>

<IconButton on:click={openDialog} class="material-icons">build</IconButton>

<style>
  .dialog {
    margin-left: 8px;
    margin-right: 8px;
  }
</style>
