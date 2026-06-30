# docs-alfred

## docs-cli

```shell

go install github.com/xbpk3t/docs-alfred/cmd/docs-cli@main

```

```shell
docs-cli images check
docs-cli images fix
docs-cli blog check
docs-cli dotfiles check
docs-cli dotfiles sync-record
# Classify and summarize explicit URLs into wiki
docs-cli wiki add <url> [url...]
# Process wiki/inbox.md and flush handled lines
docs-cli wiki inbox process
```


## data-cli

```shell

go install github.com/xbpk3t/docs-alfred/cmd/data-cli@main

```

```shell
data-cli render
data-cli check gh
data-cli dedup gh
```


## rss2nl

```shell

go install github.com/xbpk3t/docs-alfred/cmd/rss2nl@main

```


## linear2nl


```shell

go install github.com/xbpk3t/docs-alfred/cmd/linear2nl@main

```

---

```shell
linear2nl export --days 2 --format md
mv linear2nl_*.md /tmp/
```



## Others

### gh-alfred

```shell

go install github.com/xbpk3t/docs-alfred/cmd/gh-alfred@main

```

```shell
gh-alfred search <query>
gh-alfred sync
gh-alfred export
gh-alfred validate
```


### pwgen

```shell

go install github.com/xbpk3t/docs-alfred/cmd/pwgen@main

```
