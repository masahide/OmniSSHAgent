<script>
    import AddKeys from "./AddKeys.svelte";
    import Loadedkeys from "./Loadedkeys.svelte";
    import Settings from "./Settings.svelte";
    import Paper, { Title, Content } from "@smui/paper";
    import Button, { Label, Icon } from "@smui/button";
    import Card from "@smui/card";
    import TabBar from "@smui/tab-bar";
    import Tab from "@smui/tab";
    import { SvelteToast } from "@zerodevx/svelte-toast";

    let name = "";
    let tabs = [
        {
            icon: "support_agent",
            label: "Loaded keys",
            component: Loadedkeys,
        },
        {
            icon: "key",
            label: "Add keys",
            component: AddKeys,
        },
        {
            icon: "settings",
            label: "Settings",
            component: Settings,
        },
    ];
    let active = tabs[0];

    function exit() {
        window.go.main.App.Quit();
    }
    const toastOptions = {
        duration: 4000, // duration of progress bar tween to the `next` value
        initial: 1, // initial progress bar value
        next: 0, // next progress value
        pausable: false, // pause progress bar tween on mouse hover
        dismissable: true, // allow dismiss with close button
        reversed: false, // insert new toast to bottom of stack
        intro: { x: 256 }, // toast intro fly animation settings
        theme: {}, // css var overrides
        classes: [], // user-defined classes
    };
</script>

<main>
    <div id="input" data-wails-no-drag>
        <div>
            <TabBar {tabs} let:tab bind:active>
                <Tab {tab}>
                    <Icon class="material-icons">{tab.icon}</Icon>
                    <Label>{tab.label}</Label>
                </Tab>
            </TabBar>
        </div>
        <svelte:component this={active.component} />
    </div>
    <SvelteToast {toastOptions} />
</main>

<style>
</style>
