## What does this change do?

<!-- One or two sentences. Link the issue it closes. -->

Closes #

## How was it verified?

<!-- Commands you ran, plus anything a reviewer should reproduce. -->

```sh
gofmt -l .        # must print nothing
go vet ./...
go test -race ./...
```

## Checklist

- [ ] No `.env`, credentials, keys, model files, databases or recovery snapshots committed
- [ ] Behaviour preserved for existing routes and config keys unless the change is intentional
- [ ] `docs/` updated if configuration, policy precedence or deployment changed
- [ ] New behaviour has a test, or the PR explains why it cannot be tested
