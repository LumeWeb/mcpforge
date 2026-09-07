# mcpforge

Go package for building conditional descriptions, lists, JSON schemas, tool
targets, and agent guides over a caller-defined context.

## Usage

```go
import "go.lumeweb.com/mcpforge"

const featureTunnel mcpforge.Feature = "tunnel"

type profile struct{ features mcpforge.FeatureSet }

func (p profile) FeatureSet() mcpforge.FeatureSet { return p.features }

ctx := profile{features: mcpforge.FeatureSet{featureTunnel: true}}

description := mcpforge.Static[profile]("Publish your vault").
	WhenSentence(featureTunnel, "Uploads go through a public tunnel.")

fmt.Println(description.Resolve(ctx))

schema := mcpforge.Schema().
	StringProperty("path", "File to upload", mcpforge.When(featureTunnel)).
	Required("path")

raw := schema.RawJSON(ctx.features)
```

Builders carry no terminal, protocol, or host-detection dependencies. The
caller defines the context type, supplies the predicates, and decides what
the resolved output is used for.

## Development

```sh
go build ./...
go test -race ./...
```

## License

MIT — see [LICENSE](LICENSE).
