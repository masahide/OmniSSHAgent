<script>
  import ListKeys from "./ListKeys.svelte";
  import Settings from "./Settings.svelte";
  import { SvelteToast } from "@zerodevx/svelte-toast";
  import IconButton from "@smui/icon-button";
  import Menu from "@smui/menu";
  import List, { Item, Separator, Text } from "@smui/list";

  function exit() {
    window.go.main.App.Quit();
  }
  const toastOptions = {
    duration: 4000, // duration of progress bar tween to the `next` value
    initial: 1, // initial progress bar value
    next: 0, // next progress value
    pausable: true, // pause progress bar tween on mouse hover
    dismissable: true, // allow dismiss with close button
    reversed: false, // insert new toast to bottom of stack
    intro: { x: 256 }, // toast intro fly animation settings
    theme: {}, // css var overrides
    classes: [], // user-defined classes
  };
  let menu;
</script>

<main>
  <div id="input" data-wails-no-drag>
    <div class="list">
      <div class="bar-container">
        <div class="top-bar-row">
          <section class="top-app-bar_section-align-start">
            <IconButton
              on:click={() => menu.setOpen(true)}
              class="material-icons">menu</IconButton
            >
            <span class="mdc-top-app-bar__title">Omni SSH agent</span>
          </section>
          <section class="top-app-bar_section-align-end">
            <Settings />
          </section>
        </div>
      </div>
      <ListKeys />
    </div>
    <SvelteToast {toastOptions} />
    <Menu bind:this={menu}>
      <List>
        <Item><Text>Cancel</Text></Item>
        <Separator />
        <Item on:SMUI:action={exit}><Text>Exit</Text></Item>
      </List>
    </Menu>
  </div>
</main>

<style>
  .bar-container {
    position: static;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    box-sizing: border-box;
    width: 100%;
    z-index: 4;
  }
  .top-bar-row {
    display: flex;
    position: relative;
    box-sizing: border-box;
    width: 100%;
    height: 48px;
  }
  .top-app-bar_section-align-start {
    flex: 1 1 auto;
    display: inline-flex;
    align-items: center;
    z-index: 1;
    padding: 0 4px;
    justify-content: flex-start;
    order: -1;
  }
  .top-app-bar_section-align-end {
    flex: 1 1 auto;
    display: inline-flex;
    align-items: center;
    z-index: 1;
    padding: 0 4px;
    justify-content: flex-end;
    order: 1;
  }
</style>
