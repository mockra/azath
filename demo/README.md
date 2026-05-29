# azath demo

`azath.gif` is recorded with [vhs](https://github.com/charmbracelet/vhs) using a
self-contained config under `/tmp/azath-demo`.

## Regenerate

```sh
brew install vhs   # if missing
vhs demo/demo.tape
```

`setup.sh` writes a throwaway `~/.config/azath/config.toml`, creates fake
project directories, and spins up a couple of tmux sessions so `azath list`
shows mixed running/stopped status. `teardown.sh` cleans up.
