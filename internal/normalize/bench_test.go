package normalize

import "testing"

func benchNormalizer(b *testing.B) *Normalizer {
	b.Helper()
	return New(Dictionary{
		Abbrev: map[string]string{
			"мол": "молоко", "простокв": "простоквашино", "слив": "сливочное",
			"д/кошек": "для кошек", "охл": "охлажденный",
		},
		StopWords: map[string]bool{"шт": true, "уп": true, "вес": true},
		Version:   1,
	})
}

var benchLines = []string{
	"МОЛОКО ПРОСТОКВ.3,2% 930МЛ",
	"БЛАГОЯР Шницель Венский с сыром охлажденный ЛОТ",
	"PURINA ONE Корм сух д/кошек курица/цел з 1,5КГ",
	"Масло слив. Традиционное 82,5% 180г",
	"КАРТОФЕЛЬ МЫТЫЙ ВЕС",
}

func BenchmarkName(b *testing.B) {
	n := benchNormalizer(b)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = n.Name(benchLines[i%len(benchLines)])
	}
}

func BenchmarkStem(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = Stem("охлажденный")
	}
}
