# Development

```sh
go test -race ./...
go vet ./...
go build -o bin/mylib .
python3 scripts/build-release.py
```

# Releases

The `Release` workflow builds six archives, verifies their checksums, and publishes them to `what1f/mylib-releases`. Push a version tag such as `v0.2.0` to trigger it. You can also run the workflow manually with an existing version tag.

Configure the source repository's `RELEASE_TOKEN` Actions secret with a fine-grained personal access token that grants `Contents: write` to **only `what1f/mylib-releases`**. Do not put the token in files or commit it. GitHub's default workflow token cannot publish to the other repository.

Every version is published once. Use a new tag for a new build; existing releases are not overwritten. A failed upload leaves a draft release for inspection. Remove that draft before retrying the same version.
