package storage

import "testing"

func TestSourceKeyIsContentAddressed(t *testing.T) {
	body := []byte("<html>hello</html>")
	hash := HashContent(body)

	key := SourceKey(1342, hash)
	if key != "sources/1342/"+hash+".html" {
		t.Errorf("key = %q", key)
	}

	// 같은 내용이면 같은 키다. 잡이 재시도돼도 같은 자리에 덮어써진다.
	if SourceKey(1342, HashContent(body)) != key {
		t.Error("같은 내용인데 키가 다르다")
	}
	// 내용이 다르면 키가 다르다. 개정판이 원본을 덮어쓰지 않는다.
	if SourceKey(1342, HashContent([]byte("<html>other</html>"))) == key {
		t.Error("내용이 다른데 키가 같다")
	}
	// 책이 다르면 키가 다르다.
	if SourceKey(84, hash) == key {
		t.Error("책이 다른데 키가 같다")
	}
}

// content_hash는 적재 쪽 parse.Evaluation.SourceSHA256과 같은 값이어야 한다.
// 다르면 book_sources의 UNIQUE (book_id, content_hash)가 중복을 못 막는다.
func TestHashContentMatchesSHA256Hex(t *testing.T) {
	// echo -n "abc" | shasum -a 256
	const want = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got := HashContent([]byte("abc")); got != want {
		t.Errorf("HashContent = %q, want %q", got, want)
	}
}
