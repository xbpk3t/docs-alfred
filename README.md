# docs-alfred

Go CLI tools for the docs repository.

## Available binaries

```shell
go install github.com/xbpk3t/docs-alfred/docs-cli@main
go install github.com/xbpk3t/docs-alfred/rss2nl@main
go install github.com/xbpk3t/docs-alfred/pwgen@main
```

### docs-cli

Data rendering, validation, and GitHub repository search.

```
docs-cli data render                  Render YAML data into outputs
docs-cli data <domain> check          Check a data domain (books, music, diary, gh, goods, task, ntl)
docs-cli data <domain> duplicate      Find duplicate records (books, music, gh)
docs-cli data gh find                 Search local data/gh entries
docs-cli data gh append-record        Append a record to a data/gh entry
docs-cli gh search                    Search GitHub repositories from remote gh.yml
docs-cli gh sync                      Force refresh remote gh.yml cache
docs-cli images check                 Check docs-images against data/gh
docs-cli dotfiles check               Check dotfiles/data consistency
docs-cli dotfiles sync-plan           Plan dotfiles synchronization
docs-cli blog check                   Check blog/data consistency
```

### rss2nl

RSS newsletter, transcription, source discovery, and wiki tools.

```
rss2nl send                           Merge feeds and send newsletter
rss2nl send --check                   Check feed health (no email)
rss2nl trns [source]                  Fetch transcript data
rss2nl trns check [source]            Check transcript availability
rss2nl hunt                           Discover high-quality source URLs
rss2nl wiki [urls...]                 Classify URLs into wiki
rss2nl wiki --inbox                   Process wiki/inbox.md
```

### pwgen

Deterministic password generator.
