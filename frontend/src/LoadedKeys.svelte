<script>
    import Paper, { Title, Subtitle, Content } from "@smui/paper";
    import Button, { Label } from "@smui/button";
    import List, { Item, Text, PrimaryText, SecondaryText } from "@smui/list";
    import { toast } from '@zerodevx/svelte-toast'
/*
    let keys = [];
    const loadKeys = async () => {
        await window.go.main.App.KeyList()
            .then((list) => {
                keys = list;
            })
            .catch((err) => {
                console.error("KeyList err:" + err);
                toast.push("keylist errror:" + err, {
                    theme: {
                        '--toastBackground': '#F56565',
                        '--toastBarBackground': '#C53030'
                    }
                });
            });
    };
    loadKeys();
*/
</script>

<div>
</div>

<style>
    div {
        border: 1px solid black;
        margin: 0.2em;
        padding: 0.3em;
    }
</style>
