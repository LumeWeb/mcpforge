package mcpforge

// Characterization tests pinning the description DSL's behavior against a
// neutral test context; expected strings, ordering, and gating outcomes are
// pinned exactly.

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDescBuilderStaticOnly(t *testing.T) {
	d := Static[testCtx]("Upload a file and pin it.")
	require.Equal(t, "Upload a file and pin it.", d.Resolve(testCtx{}))
}

func TestDescBuilderWhenMatches(t *testing.T) {
	d := Static[testCtx]("Preamble.").When(featFileHostInput, "must use `file`.")
	require.Equal(t, "Preamble. must use `file`.", d.Resolve(profileWith(featFileHostInput)))
}

func TestDescBuilderWhenSkipsAbsentFeature(t *testing.T) {
	d := Static[testCtx]("Preamble.").When(featFileHostInput, "must use `file`.")
	require.Equal(t, "Preamble.", d.Resolve(profileWith()))
	require.Equal(t, "Preamble.", d.Resolve(profileWith(featSourceMint)))
}

func TestDescBuilderWhenAllRequiresEveryFeature(t *testing.T) {
	d := Static[testCtx]("P").WhenAll([]Feature{featSourceURL, featSourceData}, "both.")
	require.Equal(t, "P both.", d.Resolve(profileWith(featSourceURL, featSourceData)))
	// Missing one of the required features → segment skipped.
	require.Equal(t, "P", d.Resolve(profileWith(featSourceURL)))
	require.Equal(t, "P", d.Resolve(profileWith()))
}

func TestDescBuilderWhenAnyMatchesOnOne(t *testing.T) {
	d := Static[testCtx]("P").WhenAny([]Feature{featSourceURL, featSourceData}, "relay.")
	for _, f := range []Feature{featSourceURL, featSourceData} {
		require.Equal(t, "P relay.", d.Resolve(profileWith(f)))
	}
	require.Equal(t, "P", d.Resolve(profileWith()))
}

func TestDescBuilderUnlessSkipsMatchingFeature(t *testing.T) {
	d := Static[testCtx]("P").Unless(featSinkDrop, "no drop.")
	require.Equal(t, "P no drop.", d.Resolve(profileWith()))
	require.Equal(t, "P no drop.", d.Resolve(profileWith(featSinkLocal)))
	require.Equal(t, "P", d.Resolve(profileWith(featSinkDrop)))
}

func TestDescBuilderUnlessSentenceStartsWithSentence(t *testing.T) {
	// UnlessSentence mirrors WhenSentence for the negation: it begins a new
	// sentence (". ") when the feature is absent, so an if/else pair reads with
	// the same joining language on both branches.
	d := Static[testCtx]("Call upload_file").
		WhenSentence(featFileHostInput, "Prefer the file parameter.").
		UnlessSentence(featFileHostInput, "This host has no file parameter.")
	require.Equal(t, "Call upload_file. This host has no file parameter.", d.Resolve(profileWith()))
	require.Equal(t, "Call upload_file. Prefer the file parameter.", d.Resolve(profileWith(featFileHostInput)))
}

func TestDescBuilderIfElsePattern(t *testing.T) {
	// The if/else idiom: a When for present + Unless for absent.
	d := Static[testCtx]("byte source").
		When(featSinkDrop, "with drop.").
		Unless(featSinkDrop, "with local only.")
	require.Equal(t, "byte source with drop.", d.Resolve(profileWith(featSinkDrop)))
	require.Equal(t, "byte source with local only.", d.Resolve(profileWith()))
}

func TestDescBuilderCharsMidSentence(t *testing.T) {
	// Use the *Sep variants to continue a sentence with no space separator:
	// the default separator is a space, so WhenSep("", ...) appends without one.
	d := Static[testCtx]("call upload_file").
		UnlessSep("", featFileHostInput, "with the host file argument and archive_mode=convert").
		WhenSep("", featFileHostInput, "with a convert source").
		Static(".")
	require.Equal(t, "call upload_filewith the host file argument and archive_mode=convert .",
		d.Resolve(profileWith()))
	require.Equal(t, "call upload_filewith a convert source .", d.Resolve(profileWith(featFileHostInput)))
}

func TestDescBuilderDefaultSeparatorIsSpace(t *testing.T) {
	// The first segment carries no separator; subsequent segments are joined
	// with a single space by default.
	d := Static[testCtx]("one").Static("two").When(featSourceMint, "three")
	require.Equal(t, "one two three", d.Resolve(profileWith(featSourceMint)))
}

func TestDescBuilderEmptySepJoinsDirectly(t *testing.T) {
	d := Static[testCtx]("").StaticSep("", "a").StaticSep("", "b")
	require.Equal(t, "ab", d.Resolve(testCtx{}))
}

func TestDescBuilderFirstSegmentNeverGetsSeparator(t *testing.T) {
	// The leading Static is the first segment: any separator it declares is
	// ignored because there's no prior content.
	d := Static[testCtx]("").StaticSep("X", "only")
	require.Equal(t, "only", d.Resolve(testCtx{}))
}

func TestDescBuilderOrderPreserved(t *testing.T) {
	d := Static[testCtx]("1").
		When(featSourcePath, "2a").
		When(featSourceMint, "2b").
		Static("3")
	out := d.Resolve(profileWith(featSourceMint))
	require.Equal(t, "1 2b 3", out)
}

func TestDescBuilderReusesBuilderAcrossProfiles(t *testing.T) {
	// A DescBuilder is immutable (methods return a new value), so the same
	// builder must resolve differently per profile.
	d := Static[testCtx]("P").When(featSourcePath, "path").When(featSourceMint, "mint")
	require.Equal(t, "P path", d.Resolve(profileWith(featSourcePath)))
	require.Equal(t, "P mint", d.Resolve(profileWith(featSourceMint)))
	require.Equal(t, "P", d.Resolve(profileWith()))
	// Re-resolving with the same profile yields the same result (no state).
	require.Equal(t, "P path", d.Resolve(profileWith(featSourcePath)))
}

func TestSeparatorConstants(t *testing.T) {
	require.Equal(t, "", SepNone)
	require.Equal(t, " ", SepSpace)
	require.Equal(t, ". ", SepSentence)
	require.Equal(t, "; ", SepClause)
	require.Equal(t, ", ", SepList)
	require.Equal(t, " — ", SepDash)
}

func TestWhenSentence(t *testing.T) {
	d := Static[testCtx]("pinned").WhenSentence(featSourceMint, "poll the upload handle.")
	require.Equal(t, "pinned. poll the upload handle.", d.Resolve(profileWith(featSourceMint)))
	require.Equal(t, "pinned", d.Resolve(profileWith()))
}

func TestWhenAnySentence(t *testing.T) {
	d := Static[testCtx]("pinned").WhenAnySentence([]Feature{featSourceURL, featSourceData}, "use the relay tools.")
	for _, f := range []Feature{featSourceURL, featSourceData} {
		require.Equal(t, "pinned. use the relay tools.", d.Resolve(profileWith(f)))
	}
	require.Equal(t, "pinned", d.Resolve(profileWith()))
}

func TestStaticSentence(t *testing.T) {
	d := Static[testCtx]("pinned").StaticSentence("site bundles become directory DAGs.")
	require.Equal(t, "pinned. site bundles become directory DAGs.", d.Resolve(testCtx{}))
}

func TestWhenClause(t *testing.T) {
	d := Static[testCtx]("writes to a host path").WhenClause(featSinkDrop, "drop returns a GET link.")
	require.Equal(t, "writes to a host path; drop returns a GET link.", d.Resolve(profileWith(featSinkDrop)))
	require.Equal(t, "writes to a host path", d.Resolve(profileWith()))
}

func TestWhenListAndStaticList(t *testing.T) {
	d := Static[testCtx]("use the host file").WhenList(featSourcePath, "or a path source").StaticList("not individual assets.")
	require.Equal(t, "use the host file, or a path source, not individual assets.", d.Resolve(profileWith(featSourcePath)))
	require.Equal(t, "use the host file, not individual assets.", d.Resolve(profileWith()))
}

func TestWhenDash(t *testing.T) {
	d := Static[testCtx]("no curl needed").WhenDash(featSourceMint, "the host already owns it.")
	require.Equal(t, "no curl needed — the host already owns it.", d.Resolve(profileWith(featSourceMint)))
}

func TestResolveSegments(t *testing.T) {
	d := Static[testCtx]("preamble").
		When(featSourcePath, "path clause").
		When(featSourceMint, "mint clause").
		Static("suffix")
	segs := d.ResolveSegments(profileWith(featSourcePath))
	require.Equal(t, []string{"preamble", "path clause", "suffix"}, segs)
	// Segments are returned unjoined and unsubstituted (structural view).
	segs = d.ResolveSegments(profileWith(featSourceMint))
	require.Equal(t, []string{"preamble", "mint clause", "suffix"}, segs)
}

func TestWhenRunAndUnlessRun(t *testing.T) {
	// Run joins with no separator — e.g. a trailing period appended directly.
	d := Static[testCtx]("disk").WhenRun(featSinkDrop, " ; via drop").UnlessRun(featSinkDrop, ".")
	require.Equal(t, "disk ; via drop", d.Resolve(profileWith(featSinkDrop)))
	require.Equal(t, "disk.", d.Resolve(profileWith()))
}

func TestSentences(t *testing.T) {
	d := Static[testCtx]("a.").Sentences("b.", "c.", "d.")
	got := d.Resolve(profileWith())
	require.Equal(t, "a. b. c. d.", got, "self-punctuated sentences join with single spaces, no double periods")
	require.NotContains(t, got, "..")
}

func TestSentencesWhen(t *testing.T) {
	d := Static[testCtx]("a.").Sentences("b.").SentencesWhen(featSourceMint, "c.", "d.")
	require.Equal(t, "a. b. c. d.", d.Resolve(profileWith(featSourceMint)))
	require.Equal(t, "a. b.", d.Resolve(profileWith()), "block drops entirely when the gate feature is absent")
}

func TestSentencesUnless(t *testing.T) {
	d := Static[testCtx]("a.").SentencesUnless(featSourceMint, "b.")
	require.Equal(t, "a.", d.Resolve(profileWith(featSourceMint)))
	require.Equal(t, "a. b.", d.Resolve(profileWith()))
}

func TestSentencesWhenAny(t *testing.T) {
	d := Static[testCtx]("a.").SentencesWhenAny([]Feature{featSourceMint, featSourcePath}, "b.")
	require.Equal(t, "a. b.", d.Resolve(profileWith(featSourceMint)))
	require.Equal(t, "a. b.", d.Resolve(profileWith(featSourcePath)))
	require.Equal(t, "a.", d.Resolve(profileWith(featSourceURL)))
}

func TestListBuilderNumberedDropsGatedItems(t *testing.T) {
	l := List[testCtx](ListNumbered).
		Intro("Pick the byte route in this order:").
		ItemWhen(featSourceMint, "a local file → upload_file mint + host PUT + upload_status").
		ItemWhen(featSourceURL, "a public HTTPS URL → upload_url").
		ItemWhen(featSourceData, "only raw bytes → upload_data")

	// Grok: mint + url + data all present → three contiguous items.
	grok := l.Build(grokHTTP)
	require.Equal(t, "Pick the byte route in this order:\n"+
		"1. a local file → upload_file mint + host PUT + upload_status\n"+
		"2. a public HTTPS URL → upload_url\n"+
		"3. only raw bytes → upload_data\n", grok)

	// Only mint → item 1 becomes "1." (renumbered, no gaps).
	mintOnly := l.Build(profileWith(featSourceMint))
	require.Equal(t, "Pick the byte route in this order:\n"+
		"1. a local file → upload_file mint + host PUT + upload_status\n", mintOnly)

	// Openai tunnel (url + data, no mint) → two items renumbered 1..2.
	tunnel := l.Build(profileWith(featSourceURL, featSourceData))
	require.Equal(t, "Pick the byte route in this order:\n"+
		"1. a public HTTPS URL → upload_url\n"+
		"2. only raw bytes → upload_data\n", tunnel)
}

func TestListBuilderEmptyWhenNoItems(t *testing.T) {
	l := List[testCtx](ListBulleted).Intro("Choices:").ItemWhen(featSourceData, "data")
	require.Equal(t, "", l.Build(profileWith(featSourceMint)), "no surviving items and no intro-only output")
}

func TestListBuilderBulleted(t *testing.T) {
	l := List[testCtx](ListBulleted).Item("first").Item("second")
	require.Equal(t, "- first\n- second\n", l.Build(testCtx{}))
}

func TestDescBuilderListBlock(t *testing.T) {
	l := List[testCtx](ListNumbered).
		ItemWhen(featSourceMint, "mint route").
		ItemWhen(featSourceURL, "url route")
	d := Static[testCtx]("Choose the byte route in this order:").
		ListWhenAny([]Feature{featSourceMint, featSourceURL}, l)
	got := d.Resolve(profileWith(featSourceMint, featSourceURL))
	require.Equal(t, "Choose the byte route in this order:\n1. mint route\n2. url route\n", got)

	// No matching feature → the whole list block is omitted.
	gotNone := d.Resolve(profileWith(featSourceData))
	require.Equal(t, "Choose the byte route in this order:", gotNone)
}

// TestCharacterizationUploadFileDescPathOnlyNoMintPolling is a regression
// on a path-only transport (stdio), upload_file's resolved description must
// NOT instruct the model to poll upload_status or reference the mint source
// mode — path mode is synchronous, no handle is returned, and mint is not a
// legal source.mode value. The description under test mirrors the deployed
// upload tool's DSL structure (same gate pattern); this pins the
// transport-mechanism gating behavior.
func TestCharacterizationUploadFileDescPathOnlyNoMintPolling(t *testing.T) {
	desc := Static[testCtx]("Upload a file and pin it. The wait flag waits for this upload's own pin operation.").
		When(featFileHostInput, "Use `file` when the host already has the file.").
		When(featSourceMint,
			"Use source.mode=mint to get a one-time presigned HTTP PUT endpoint, then poll upload_status with the returned upload_handle until it reports completed.").
		When(featSourcePath, "Use source.mode=path with a host-side file/directory/archive path.").
		Resolve(stdioGeneric)
	require.NotContains(t, desc, "upload_status",
		"path-only transport must not instruct polling upload_status")
	require.NotContains(t, desc, "mint",
		"path-only transport must not reference the mint source mode")
	require.Contains(t, desc, "source.mode=path",
		"path-only transport must advertise the path source mode")
}

// TestCharacterizationVaultGetFileDescNoTunnelLabel is a regression:
// vault_get_file's resolved description must not hardcode the transport name
// "tunnel". On stdio (which supports FeatSinkDrop), the description should
// advertise drop, not say "unavailable". Mirrors the deployed source DSL
// structure; pins current behavior.
func TestCharacterizationVaultGetFileDescNoTunnelLabel(t *testing.T) {
	desc := Static[testCtx](
		"Download a file from your encrypted vault by vault_path. Set sink=local to write the decrypted bytes to a host-side output_path (available on every transport)").
		WhenSep(SepSpace, featSinkDrop,
			"or sink=drop to get a one-time HTTP GET filedrop link to pull from out of band.").
		UnlessSep(SepSentence, featSinkDrop,
			"The filedrop GET sink is unavailable on this transport.").
		Resolve(stdioGeneric)
	require.NotContains(t, desc, "tunnel",
		"vault_get_file description must not hardcode 'tunnel' as the transport name")
	require.NotContains(t, desc, "unavailable on this transport",
		"stdio supports drop (FeatSinkDrop) so the unavailable clause must not render")
	require.Contains(t, desc, "sink=drop",
		"vault_get_file description on stdio should advertise the drop sink")
}

// TestCharacterizationDownloadFileDescStdioAdvertisesDrop is a regression:
// on stdio, the download_file description must advertise the drop sink (stdio
// can spin up a local HTTP listener for the filedrop), not say it is
// unavailable. Mirrors the deployed source DSL structure; pins current
// behavior.
func TestCharacterizationDownloadFileDescStdioAdvertisesDrop(t *testing.T) {
	desc := Static[testCtx](
		"Download content as a file. Set sink=local to write the bytes to a host-side output_path (available on every transport)").
		WhenSep(SepSpace, featSinkDrop,
			"or sink=drop to get a one-time HTTP GET filedrop link to pull from out of band (curl -o <url> or a browser link).").
		UnlessSep(SepSentence, featSinkDrop,
			"The filedrop GET sink is unavailable on this transport.").
		Resolve(stdioGeneric)
	require.NotContains(t, desc, "unavailable on this transport",
		"stdio supports drop so the unavailable clause must not render")
	require.Contains(t, desc, "sink=drop",
		"download_file description on stdio should advertise the drop sink")
}

// TestCharacterizationDescBuilderSurfaceGating verifies the surface-domain
// gating helpers' generic replacement (WhenPred over a context predicate)
// fires on domain availability (Surface accessor VaultOn style), independent
// of deployment.
func TestCharacterizationDescBuilderSurfaceGating(t *testing.T) {
	d := Static[testCtx]("P").WhenPred(testSurfaceIs(testSurface.vaultOn), "vault on.")
	require.Equal(t, "P vault on.", d.Resolve(testCtx{Surface: fullTestSurface}))
	require.Equal(t, "P", d.Resolve(testCtx{Surface: hostedTestSurface}))

	u := Static[testCtx]("P").UnlessPred(testSurfaceIs(testSurface.vaultOn), "no vault.")
	require.Equal(t, "P no vault.", u.Resolve(testCtx{Surface: hostedTestSurface}))
	require.Equal(t, "P", u.Resolve(testCtx{Surface: fullTestSurface}))
}

// TestCharacterizationDescBuilderHostedGating verifies deployment gating
// fires on the context's Hosted field (generic replacement for
// WhenHosted/UnlessHosted), independent of the domain surface.
func TestCharacterizationDescBuilderHostedGating(t *testing.T) {
	d := Static[testCtx]("P").WhenPred(testHostedIs(true), "hosted.").
		UnlessPred(testHostedIs(true), "local.")
	require.Equal(t, "P hosted.", d.Resolve(testCtx{Hosted: true}))
	require.Equal(t, "P local.", d.Resolve(testCtx{Hosted: false}))
	require.Equal(t, "P hosted.", d.Resolve(testCtx{Surface: hostedTestSurface, Hosted: true}))
}
