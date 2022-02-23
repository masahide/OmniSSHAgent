<script>
    import Paper, { Title, Subtitle, Content } from "@smui/paper";
    import Card from "@smui/card";
    import Button, { Label } from "@smui/button";
    import AddFileDialog from "./AddFileDialog.svelte";

    let files = [];
    function handleData(event) {
        let data = event.detail.data;
        console.log(data);
        alert(data);
        window.runtime.LogDebug(JSON.stringify(data));
    }
</script>

<AddFileDialog on:data={handleData} />

<style>
    /*
    div {
        border: 1px solid black;
        margin: 0.2em;
        padding: 0.3em;
    }
    */
</style>
