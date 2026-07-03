package autopot

import (
	"image"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
)

// ---------------------------------------------------------------------------
// Benchmark helpers
// ---------------------------------------------------------------------------

// benchFixture loads a testdata fixture once (no testing.T for benchmark).
func benchFixture(name string) image.Image {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "testdata")
	path := filepath.Join(dir, name)
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		panic(err)
	}
	return img
}

// benchMapped loads the fixture, detects bars, and panics on failure.
func benchMapped(name string) (image.Image, MappedBars) {
	img := benchFixture(name)
	mapped, err := RefreshBarPair(img)
	if err != nil {
		panic(err)
	}
	return img, mapped
}

// allBenchFixtures returns the sorted list of known fixture files for benchmarks.
func allBenchFixtures() []string {
	files := make([]string, 0, len(knownBarCases()))
	for name := range knownBarCases() {
		files = append(files, name)
	}
	sort.Strings(files)
	return files
}

// ---------------------------------------------------------------------------
// Individual function benchmarks (per fixture)
// ---------------------------------------------------------------------------

func BenchmarkRefreshBarPair(b *testing.B) {
	imgs := make(map[string]image.Image, len(allBenchFixtures()))
	for _, name := range allBenchFixtures() {
		imgs[name] = benchFixture(name)
	}
	for _, name := range allBenchFixtures() {
		img := imgs[name]
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = RefreshBarPair(img)
			}
		})
	}
}

func BenchmarkReadHPFill(b *testing.B) {
	type hpCase struct {
		img image.Image
		hp  Rect
	}
	var cases []hpCase
	for _, name := range allBenchFixtures() {
		img, mapped := benchMapped(name)
		cases = append(cases, hpCase{img: img, hp: mapped.HP})
	}
	for i, c := range cases {
		name := allBenchFixtures()[i]
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				ReadHPFill(c.img, c.hp)
			}
		})
	}
}

func BenchmarkReadSPFill(b *testing.B) {
	type spCase struct {
		img image.Image
		sp  Rect
	}
	var cases []spCase
	for _, name := range allBenchFixtures() {
		img, mapped := benchMapped(name)
		cases = append(cases, spCase{img: img, sp: mapped.SP})
	}
	for i, c := range cases {
		name := allBenchFixtures()[i]
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				ReadSPFill(c.img, c.sp)
			}
		})
	}
}

func BenchmarkBarLooksFull(b *testing.B) {
	type fullCase struct {
		img   image.Image
		rect  Rect
		hpBar bool
	}
	var cases []fullCase
	for _, name := range allBenchFixtures() {
		img, mapped := benchMapped(name)
		cases = append(cases, fullCase{img: img, rect: mapped.HP, hpBar: true})
		cases = append(cases, fullCase{img: img, rect: mapped.SP, hpBar: false})
	}
	for i, c := range cases {
		label := map[bool]string{true: "HP", false: "SP"}[c.hpBar]
		b.Run(label+"["+allBenchFixtures()[i/2]+"]", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				BarLooksFull(c.img, c.rect, c.hpBar)
			}
		})
	}
}

func BenchmarkBestFillWidth(b *testing.B) {
	type fwCase struct {
		img   image.Image
		rect  Rect
		hpBar bool
	}
	var cases []fwCase
	for _, name := range allBenchFixtures() {
		img, mapped := benchMapped(name)
		cases = append(cases, fwCase{img: img, rect: mapped.HP, hpBar: true})
		cases = append(cases, fwCase{img: img, rect: mapped.SP, hpBar: false})
	}
	for i, c := range cases {
		label := map[bool]string{true: "HP", false: "SP"}[c.hpBar]
		b.Run(label+"["+allBenchFixtures()[i/2]+"]", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				bestFillWidth(c.img, c.rect, c.hpBar)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Full pipeline benchmark
// ---------------------------------------------------------------------------

func BenchmarkFullBarReadPipeline(b *testing.B) {
	type pipelineCase struct {
		img image.Image
	}
	var cases []pipelineCase
	for _, name := range allBenchFixtures() {
		img := benchFixture(name)
		cases = append(cases, pipelineCase{img: img})
	}
	for i, c := range cases {
		name := allBenchFixtures()[i]
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				mapped, err := RefreshBarPair(c.img)
				if err != nil {
					b.Fatal(err)
				}
				ReadHPFill(c.img, mapped.HP)
				ReadSPFill(c.img, mapped.SP)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Composite benchmark: ReadMappedBars (HP+SP fill + normalise)
// ---------------------------------------------------------------------------

func BenchmarkReadMappedBars(b *testing.B) {
	type mc struct {
		img    image.Image
		mapped MappedBars
	}
	var cases []mc
	for _, name := range allBenchFixtures() {
		img, mapped := benchMapped(name)
		cases = append(cases, mc{img: img, mapped: mapped})
	}
	for i, c := range cases {
		name := allBenchFixtures()[i]
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				ReadMappedBars(c.img, c.mapped)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Stabilizer benchmark (UpdatePair — full hot path with mu)
// ---------------------------------------------------------------------------

func BenchmarkStabilizerUpdatePair(b *testing.B) {
	type stabCase struct {
		img    image.Image
		mapped MappedBars
	}
	var cases []stabCase
	for _, name := range allBenchFixtures() {
		img, mapped := benchMapped(name)
		cases = append(cases, stabCase{img: img, mapped: mapped})
	}

	for _, hpBar := range []bool{true, false} {
		label := map[bool]string{true: "HP", false: "SP"}[hpBar]
		for i, c := range cases {
			name := allBenchFixtures()[i]
			b.Run(label+"["+name+"]", func(b *testing.B) {
				s := NewBarStabilizer(hpBar, 50)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					s.UpdatePair(c.img, hpBar, c.mapped, true)
				}
			})
		}
	}
}
