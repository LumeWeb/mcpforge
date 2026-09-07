package mcpforge

// Test support: a neutral platform-profile analogue plus test-local feature
// and surface vocabulary. It carries the same fields a deployed host profile
// exposes (Features, Transport, Host, Surface, Hosted) so the
// characterization tests keep their expectations verbatim while never
// importing a host package — they pin the DSL's semantics against any
// FeatureCarrier context.

// Test-local feature vocabulary mirroring the capability names the original
// characterization tests gate on.
const (
	featFileHostInput Feature = "file-host-input"
	featSourcePath    Feature = "source-path"
	featSourceMint    Feature = "source-mint"
	featSourceURL     Feature = "source-url"
	featSourceData    Feature = "source-data"
	featSinkLocal     Feature = "sink-local"
	featSinkDrop      Feature = "sink-drop"
	featMCPApps       Feature = "mcp-apps-ui"
)

type testHost string
type testTransport string

const (
	hostGrok    testHost    = "grok"
	hostGeneric testHost    = "generic"
	hostChatGPT testHost    = "chatgpt"
)

const (
	transportHTTP   testTransport = "http"
	transportStdio  testTransport = "stdio"
	transportOpenAI testTransport = "openai"
)

// testSurface mirrors a domain-availability surface: the zero value is the
// FULL surface so unconstrained fixtures stay unlocked, matching deployed
// accessor semantics (zero surface = all on).
type testSurface struct {
	Vault    bool
	Pins     bool
	Websites bool
}

func (s testSurface) isZero() bool { return s == testSurface{} }

func (s testSurface) flagOn(set bool) bool {
	if s.isZero() {
		return true
	}
	return set
}

func (s testSurface) vaultOn() bool { return s.flagOn(s.Vault) }
func (s testSurface) pinsOn() bool  { return s.flagOn(s.Pins) }

var fullTestSurface = testSurface{Vault: true, Pins: true, Websites: true}

var hostedTestSurface = testSurface{Pins: true, Websites: true}

// testCtx is the neutral platform profile analogue. It satisfies
// FeatureCarrier via a value-receiver FeatureSet method.
type testCtx struct {
	Host      testHost
	Transport testTransport
	Features  FeatureSet
	Surface   testSurface
	Hosted    bool
}

func (c testCtx) FeatureSet() FeatureSet { return c.Features }

// Profile test fixtures mirroring deployed host profiles, with
// transport-mechanism features derived the same way (HTTP → mint+drop, stdio
// → path+drop+co-located, tunnel → url+data).

var grokHTTP = testCtx{
	Host:      hostGrok,
	Transport: transportHTTP,
	Features: featureSet(
		featSourceData, featSourceURL, // Grok-declared relay-tool capabilities
		featSourceMint, featSinkLocal, featSinkDrop, "remote-access", // HTTP transport mechanisms
	),
}

var openAITunnel = testCtx{
	Host:      hostChatGPT,
	Transport: transportOpenAI,
	Features: featureSet(
		featFileHostInput, featMCPApps, // host-declared capabilities
		featSourceURL, featSourceData, featSinkLocal, // tunnel transport mechanisms
	),
}

var stdioGeneric = testCtx{
	Host:      hostGeneric,
	Transport: transportStdio,
	Features: featureSet(
		featSourcePath, featSinkLocal, featSinkDrop, // stdio transport mechanisms
	),
}

// profileWith returns a testCtx carrying exactly the given features.
func profileWith(features ...Feature) testCtx {
	fs := make(FeatureSet, len(features))
	for _, f := range features {
		fs[f] = true
	}
	return testCtx{Features: fs}
}

// Predicate constructors used by the characterization tests to gate prose on
// owned predicates.
func testHostIs(h testHost) Predicate[testCtx] {
	return func(p testCtx) bool { return p.Host == h }
}

func testTransportIs(t testTransport) Predicate[testCtx] {
	return func(p testCtx) bool { return p.Transport == t }
}

func testHostedIs(hosted bool) Predicate[testCtx] {
	return func(p testCtx) bool { return p.Hosted == hosted }
}

func testSurfaceIs(get func(testSurface) bool) Predicate[testCtx] {
	return func(p testCtx) bool { return get(p.Surface) }
}

// cloneFeatures returns a shallow copy of the context with a cloned
// FeatureSet.
func cloneFeatures(p testCtx) testCtx {
	p.Features = p.Features.Clone()
	return p
}
