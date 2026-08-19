package parse

// Version은 추출 결과가 달라질 수 있는 변경마다 올린다.
// 전략·후처리·정규화·stable_id 규칙이 대상이다. 리포트 서식만 바뀌면 올리지 않는다.
//
// 올리는 것을 잊으면 같은 parser_version으로 다른 결과가 저장된다.
// UNIQUE (book_id, source_id, parser_version)이 그걸 막아 두 번째 적재가 무시되므로,
// 파서를 고쳤는데 결과가 안 바뀌면 이 상수부터 의심할 것.
const Version = "2026-08-19"
