package protocol_test

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"maps"
	"math/big"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/tuanldas/rterm/internal/protocol"
)

// Bộ sinh test vector của docs/protocol/examples. Mọi khoá trong ví dụ là khoá
// thử nghiệm công khai suy ra tất định từ một hạt giống cố định (testKeys), nên
// ai cũng sinh lại được và chúng không bảo vệ gì. Ed25519 vốn ký tất định, ECDSA
// ký theo RFC 6979, nên cùng đầu vào luôn ra cùng byte và ví dụ đã commit được
// so từng byte với kết quả của bộ sinh.
//
// Khi đầu vào của chữ ký đổi (thêm trường vào PolicySnapshot.full, đổi nonce…)
// thì sinh lại:
//
//	go test ./internal/protocol -run TestViDuCoChuKyKhopBoSinh -update
//
// Bộ sinh chỉ thay giá trị và giữ nguyên mọi byte khác của tệp (thụt dòng, thứ
// tự trường, xuống dòng cuối tệp), nên diff chỉ gồm giá trị thật sự đổi.
var update = flag.Bool("update", false, "ghi lại khoá, payload và chữ ký trong docs/protocol/examples theo bộ sinh")

type keyAlg int

const (
	algEd25519 keyAlg = iota
	algECDSAP256
)

// testKey là một khoá thử nghiệm của bộ ví dụ.
type testKey struct {
	// seed là hạt giống trung tính: không chứa tên sản phẩm, để đổi tên không
	// làm đổi test vector (TestTenSanPhamChiKhaiBaoMotCho cũng quét chuỗi này).
	seed string
	alg  keyAlg

	// anchor là nơi khai báo khoá công khai, dạng "<miền>/<ví dụ>#<JSON pointer>".
	// Bộ sinh đặt khoá suy ra vào đây rồi thay mọi bản sao của khoá cũ và vân tay
	// của nó ở các ví dụ khác, nên đổi hạt giống chỉ cần chạy lại -update.
	anchor string
}

// Hạt giống được ghi ở docs/protocol/sync.md, mục Test vector.
var (
	keyPolicy1 = &testKey{"test-vector/policy-key-1", algEd25519, "http-api/EnrollPollResult.full#/policy_keys/0/public_key"}
	keyPolicy2 = &testKey{"test-vector/policy-key-2", algEd25519, "sync/KeyEndorsement.full#/public_key"}
	keyHost    = &testKey{"test-vector/host-key", algEd25519, "http-api/EnrollStartRequest.full#/host_key"}
	keyDevice  = &testKey{"test-vector/device-key", algECDSAP256, "http-api/RegisterKeyRequest.device#/public_key"}
	keyOwner   = &testKey{"test-vector/ssh-key-owner", algEd25519, "sync/PolicySnapshot.full#/grants/0/keys/0/public_key"}
	keyMember  = &testKey{"test-vector/ssh-key-member", algEd25519, "sync/PolicySnapshot.full#/grants/1/keys/0/public_key"}

	testKeys = []*testKey{keyPolicy1, keyPolicy2, keyHost, keyDevice, keyOwner, keyMember}
)

const (
	snapshotExample = "sync/PolicySnapshot.full"

	// envelopePayload là base64 của snapshotExample ở dạng JSON gọn: bỏ khoảng
	// trắng ngoài chuỗi, giữ nguyên thứ tự trường.
	envelopePayload = "sync/SignedEnvelope.full#/payload"
)

// signedValue là một chữ ký trong ví dụ: trường chứa nó, khoá ký, ngữ cảnh, và
// thông điệp dựng đúng như bảng "Mã hoá chữ ký" của docs/protocol/sync.md.
type signedValue struct {
	at      string
	key     *testKey
	context protocol.SigningContext
	message func(f exampleSet) ([]byte, error)
}

var signedValues = []signedValue{
	{"sync/SignedEnvelope.full#/sig", keyPolicy1, protocol.ContextPolicySnapshot, func(f exampleSet) ([]byte, error) {
		return f.bytesAt(envelopePayload)
	}},
	{"sync/KeyEndorsement.full#/sig", keyPolicy1, protocol.ContextPolicyKeyRollover, func(f exampleSet) ([]byte, error) {
		var e protocol.KeyEndorsement
		if err := json.Unmarshal(f["sync/KeyEndorsement.full"], &e); err != nil {
			return nil, fmt.Errorf("sync/KeyEndorsement.full: %w", err)
		}
		return keyRolloverMessage(e.KeyID, e.PublicKey, uint64(e.NotBefore)), nil
	}},
	{"sync/SyncAuth.full#/sig", keyHost, protocol.ContextSyncAuth, func(f exampleSet) ([]byte, error) {
		nonce, err := f.bytesAt("sync/SyncHelloResult.full#/nonce")
		if err != nil {
			return nil, err
		}
		machineID, err := f.str("sync/SyncHello.full#/machine_id")
		if err != nil {
			return nil, err
		}
		return syncAuthMessage(nonce, exampleOrigin, machineID, protocol.SyncProto), nil
	}},
	{"http-api/EnrollPollRequest.full#/sig", keyHost, protocol.ContextEnrollPoP, func(f exampleSet) ([]byte, error) {
		deviceCode, err := f.str("http-api/EnrollPollRequest.full#/device_code")
		if err != nil {
			return nil, err
		}
		return enrollPoPMessage(deviceCode, exampleOrigin), nil
	}},
	{"http-api/RegisterKeyRequest.device#/sig", keyDevice, protocol.ContextDeviceKeyPoP, func(f exampleSet) ([]byte, error) {
		nonce, err := f.bytesAt("http-api/RegisterKeyRequest.device#/nonce")
		if err != nil {
			return nil, err
		}
		return deviceKeyPoPMessage(nonce, exampleOrigin), nil
	}},
}

func (k *testKey) ed25519Key() ed25519.PrivateKey {
	seed := sha256.Sum256([]byte(k.seed))
	return ed25519.NewKeyFromSeed(seed[:])
}

func (k *testKey) ecdsaKey() (*ecdsa.PrivateKey, error) {
	// Từ Go 1.26 ecdsa.GenerateKey bỏ qua nguồn ngẫu nhiên truyền vào, nên khoá
	// tất định phải dựng thẳng từ số bí mật.
	d := sha256.Sum256([]byte(k.seed))
	priv, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), d[:])
	if err != nil {
		return nil, fmt.Errorf("hạt giống %q không cho số bí mật P-256 hợp lệ: %w", k.seed, err)
	}
	return priv, nil
}

// publicKey trả dòng authorized_keys và vân tay của khoá.
func (k *testKey) publicKey() (line, fp string, err error) {
	var typ string
	var blob []byte
	switch k.alg {
	case algEd25519:
		typ = "ssh-ed25519"
		pub := k.ed25519Key().Public().(ed25519.PublicKey)
		blob = slices.Concat(wireString([]byte(typ)), wireString(pub))
	case algECDSAP256:
		priv, err := k.ecdsaKey()
		if err != nil {
			return "", "", err
		}
		point, err := priv.PublicKey.Bytes()
		if err != nil {
			return "", "", fmt.Errorf("mã hoá khoá công khai P-256: %w", err)
		}
		typ = "ecdsa-sha2-nistp256"
		blob = slices.Concat(wireString([]byte(typ)), wireString([]byte("nistp256")), wireString(point))
	default:
		return "", "", fmt.Errorf("kiểu khoá %d chưa hỗ trợ", k.alg)
	}
	return typ + " " + base64.StdEncoding.EncodeToString(blob), fingerprint(blob), nil
}

// sign ký ngữ_cảnh ‖ 0x00 ‖ msg và trả chữ ký dạng SSH đã mã hoá base64.
func (k *testKey) sign(ctx protocol.SigningContext, msg []byte) (string, error) {
	data := slices.Concat([]byte(ctx), []byte{0}, msg)
	var format string
	var blob []byte
	switch k.alg {
	case algEd25519:
		format, blob = "ssh-ed25519", ed25519.Sign(k.ed25519Key(), data)
	case algECDSAP256:
		priv, err := k.ecdsaKey()
		if err != nil {
			return "", err
		}
		digest := sha256.Sum256(data)
		// Nguồn ngẫu nhiên nil: ký tất định theo RFC 6979, nên chạy lại -update
		// không đổi chữ ký.
		der, err := priv.Sign(nil, digest[:], crypto.SHA256)
		if err != nil {
			return "", fmt.Errorf("ký ECDSA: %w", err)
		}
		var rs struct{ R, S *big.Int }
		rest, err := asn1.Unmarshal(der, &rs)
		if err != nil {
			return "", fmt.Errorf("chữ ký ECDSA không phải DER: %w", err)
		}
		if len(rest) > 0 {
			return "", errors.New("chữ ký ECDSA có byte thừa sau DER")
		}
		format, blob = "ecdsa-sha2-nistp256", slices.Concat(mpint(rs.R), mpint(rs.S))
	default:
		return "", fmt.Errorf("kiểu khoá %d chưa hỗ trợ", k.alg)
	}
	return base64.StdEncoding.EncodeToString(slices.Concat(wireString([]byte(format)), wireString(blob))), nil
}

// mpint mã hoá số không âm theo RFC 4251 §5: big-endian tối giản, thêm byte 0
// khi bit cao nhất bật để số không bị đọc thành âm.
func mpint(n *big.Int) []byte {
	b := n.Bytes()
	if len(b) > 0 && b[0]&0x80 != 0 {
		b = append([]byte{0}, b...)
	}
	return wireString(b)
}

// fingerprint là vân tay của blob khoá, giống ssh-keygen -lf.
func fingerprint(blob []byte) string {
	sum := sha256.Sum256(blob)
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
}

// authorizedKey bỏ ghi chú của dòng authorized_keys, giữ "<kiểu> <base64>".
func authorizedKey(line string) string {
	if f := strings.Fields(line); len(f) >= 2 {
		return f[0] + " " + f[1]
	}
	return line
}

// exampleSet là toàn bộ ví dụ dạng byte, khoá "<miền>/<ví dụ>" (tên tệp bỏ .json).
type exampleSet map[string][]byte

// str đọc chuỗi khác rỗng tại ref "<miền>/<ví dụ>#<JSON pointer>". Pointer theo
// RFC 6901 nhưng không hỗ trợ ký tự thoát, vì tên trường của hợp đồng không chứa
// "/" hay "~".
func (f exampleSet) str(ref string) (string, error) {
	name, pointer, _ := strings.Cut(ref, "#")
	raw, ok := f[name]
	if !ok {
		return "", fmt.Errorf("%s: không có ví dụ %s", ref, name)
	}
	if !strings.HasPrefix(pointer, "/") {
		return "", fmt.Errorf("%s: JSON pointer phải bắt đầu bằng /", ref)
	}
	var node any
	if err := json.Unmarshal(raw, &node); err != nil {
		return "", fmt.Errorf("%s: ví dụ không phải JSON: %w", ref, err)
	}
	for tok := range strings.SplitSeq(pointer[1:], "/") {
		switch v := node.(type) {
		case map[string]any:
			node, ok = v[tok]
		case []any:
			i, err := strconv.Atoi(tok)
			ok = err == nil && i >= 0 && i < len(v)
			if ok {
				node = v[i]
			}
		default:
			ok = false
		}
		if !ok {
			return "", fmt.Errorf("%s: không có phần tử %q", ref, tok)
		}
	}
	s, ok := node.(string)
	if !ok || s == "" {
		return "", fmt.Errorf("%s: cần chuỗi khác rỗng", ref)
	}
	return s, nil
}

// bytesAt đọc trường base64 chuẩn tại ref.
func (f exampleSet) bytesAt(ref string) ([]byte, error) {
	s, err := f.str(ref)
	if err != nil {
		return nil, err
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("%s: không phải base64 chuẩn: %w", ref, err)
	}
	return b, nil
}

// replaceAll thay giá trị from bằng to ở mọi ví dụ. Chỉ thay chuỗi JSON đúng
// bằng from, hoặc bắt đầu bằng from rồi một dấu cách (dòng authorized_keys kèm
// ghi chú), để không đụng tới chuỗi khác tình cờ chứa from.
func (f exampleSet) replaceAll(from, to string) {
	for name, raw := range f {
		raw = bytes.ReplaceAll(raw, []byte(`"`+from+`"`), []byte(`"`+to+`"`))
		f[name] = bytes.ReplaceAll(raw, []byte(`"`+from+` `), []byte(`"`+to+` `))
	}
}

// set đặt value vào trường tại ref, thay theo mọi bản sao của giá trị cũ (ví dụ
// không hợp lệ chép chữ ký hoặc payload của ví dụ hợp lệ), và cho biết có đổi
// gì không. part lấy phần của giá trị đang có để so với value: khoá bỏ ghi chú,
// giá trị khác so nguyên vẹn (wholeValue).
func (f exampleSet) set(ref, value string, part func(string) string) (bool, error) {
	cur, err := f.str(ref)
	if err != nil {
		return false, err
	}
	old := part(cur)
	if old == value {
		return false, nil
	}
	f.replaceAll(old, value)
	if cur, err = f.str(ref); err != nil {
		return false, err
	}
	// Giá trị viết với ký tự thoát của JSON thì không khớp khi thay theo byte;
	// báo lỗi thay vì để ví dụ lệch mà bộ sinh tưởng đã sửa.
	if part(cur) != value {
		return false, fmt.Errorf("%s: không thay được giá trị cũ (chuỗi có ký tự thoát của JSON?)", ref)
	}
	return true, nil
}

func wholeValue(s string) string { return s }

// placement là một vị trí bộ sinh quản, cùng giá trị bộ sinh tính cho nó.
type placement struct {
	ref  string
	want string
	fp   string              // vân tay của want khi vị trí là nơi khai báo khoá
	part func(string) string // như tham số part của set
}

// checkPlacements bảo đảm sau khi sinh, mỗi vị trí bộ sinh quản mang đúng giá trị
// tính cho chính nó. set thay theo mọi bản sao của giá trị cũ, nên khi hai vị trí
// từng giữ cùng một giá trị (chép một ví dụ có sẵn là cách thêm ví dụ tự nhiên
// nhất) thì vị trí xử lý sau ghi đè vị trí trước. Lần sinh sau đổi qua rồi đổi lại,
// cho ra đúng các byte đó, nên không kiểm ở đây thì test xanh trên ví dụ sai.
func checkPlacements(f exampleSet, placed []placement) error {
	for _, p := range placed {
		cur, err := f.str(p.ref)
		if err != nil {
			return err
		}
		got := p.part(cur)
		if got == p.want {
			continue
		}
		// Bản sao không mang dấu vết bản gốc, nên bộ sinh không tự gỡ được: in
		// giá trị đúng của mọi vị trí liên quan để người viết ví dụ sửa tay.
		involved := []placement{p}
		for _, q := range placed {
			if q.ref != p.ref && q.want == got {
				involved = append(involved, q)
			}
		}
		var lines []string
		for _, q := range involved {
			line := "  " + q.ref + ": " + q.want
			if q.fp != "" {
				line += " (vân tay " + q.fp + ")"
			}
			lines = append(lines, line)
		}
		return fmt.Errorf("không ghi ví dụ nào: %s bị ghi đè bằng giá trị của vị trí khác. Lúc sinh, các vị trí dưới đây giữ cùng một giá trị (thường do chép một ví dụ có sẵn); bộ sinh thay theo mọi bản sao nên không biết bản nào thuộc vị trí nào. Sửa tay từng vị trí thành đúng giá trị in dưới đây (khoá thì sửa cả vân tay đi kèm) rồi chạy lại -update:\n%s",
			p.ref, strings.Join(lines, "\n"))
	}
	return nil
}

// regenerate trả bộ ví dụ sau khi bộ sinh đặt lại khoá, payload và chữ ký, kèm
// mô tả những chỗ đã đổi. Thứ tự là bắt buộc: payload chứa khoá công khai, còn
// chữ ký phủ lên payload và lên khoá được chứng thực.
func regenerate(committed exampleSet) (exampleSet, []string, error) {
	f := maps.Clone(committed)
	var changes []string
	var placed []placement

	for _, k := range testKeys {
		line, fp, err := k.publicKey()
		if err != nil {
			return nil, nil, err
		}
		cur, err := f.str(k.anchor)
		if err != nil {
			return nil, nil, err
		}
		oldFP, err := fingerprintOfLine(authorizedKey(cur))
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", k.anchor, err)
		}
		changed, err := f.set(k.anchor, line, authorizedKey)
		if err != nil {
			return nil, nil, err
		}
		if changed {
			f.replaceAll(oldFP, fp)
			changes = append(changes, fmt.Sprintf("%s: khoá không suy ra từ hạt giống %q", k.anchor, k.seed))
		}
		placed = append(placed, placement{ref: k.anchor, want: line, fp: fp, part: authorizedKey})
	}

	var snapshot bytes.Buffer
	if err := json.Compact(&snapshot, f[snapshotExample]); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", snapshotExample, err)
	}
	payload := protocol.EncodePayload(snapshot.Bytes())
	changed, err := f.set(envelopePayload, payload, wholeValue)
	if err != nil {
		return nil, nil, err
	}
	if changed {
		changes = append(changes, fmt.Sprintf("%s: không phải %s ở dạng JSON gọn", envelopePayload, snapshotExample))
	}
	placed = append(placed, placement{ref: envelopePayload, want: payload, part: wholeValue})

	for _, s := range signedValues {
		msg, err := s.message(f)
		if err != nil {
			return nil, nil, fmt.Errorf("thông điệp của %s: %w", s.at, err)
		}
		sig, err := s.key.sign(s.context, msg)
		if err != nil {
			return nil, nil, fmt.Errorf("ký %s: %w", s.at, err)
		}
		changed, err := f.set(s.at, sig, wholeValue)
		if err != nil {
			return nil, nil, err
		}
		if changed {
			changes = append(changes, fmt.Sprintf("%s: không phải chữ ký %s của khoá từ hạt giống %q", s.at, s.context, s.key.seed))
		}
		placed = append(placed, placement{ref: s.at, want: sig, part: wholeValue})
	}
	if err := checkPlacements(f, placed); err != nil {
		return nil, nil, err
	}
	return f, changes, nil
}

// fingerprintOfLine tính vân tay của dòng "<kiểu> <base64>".
func fingerprintOfLine(line string) (string, error) {
	_, b64, ok := strings.Cut(line, " ")
	if !ok {
		return "", fmt.Errorf("khoá %q không có dạng '<kiểu> <base64>'", line)
	}
	blob, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", fmt.Errorf("blob khoá không phải base64: %w", err)
	}
	return fingerprint(blob), nil
}

func TestViDuCoChuKyKhopBoSinh(t *testing.T) {
	committed, paths := exampleSet{}, map[string]string{}
	for _, ex := range loadExamples(t) {
		name := ex.Domain + "/" + strings.TrimSuffix(ex.name(), ".json")
		committed[name], paths[name] = ex.Raw, ex.Path
	}
	want, changes, err := regenerate(committed)
	if err != nil {
		t.Fatalf("bộ sinh: %v", err)
	}

	var stale []string
	for _, name := range slices.Sorted(maps.Keys(want)) {
		if bytes.Equal(want[name], committed[name]) {
			continue
		}
		if !*update {
			stale = append(stale, paths[name])
			continue
		}
		if err := os.WriteFile(paths[name], want[name], 0o644); err != nil {
			t.Fatalf("ghi %s: %v", paths[name], err)
		}
		t.Logf("đã ghi lại %s", paths[name])
	}
	if len(stale) > 0 {
		t.Errorf("ví dụ lệch bộ sinh:\n  %s\ntệp cần ghi lại: %s\nsinh lại bằng: go test ./internal/protocol -run %s -update",
			strings.Join(changes, "\n  "), strings.Join(stale, ", "), t.Name())
	}

	checkOnlyGeneratedMaterial(t, want)
}

// sshKeyLine nhận dòng authorized_keys (có thể kèm ghi chú). Blob khoá SSH luôn
// mở đầu bằng độ dài của tên kiểu (ba byte 0) nên base64 của nó bắt đầu bằng AAAA.
var sshKeyLine = regexp.MustCompile(`^(ssh|ecdsa|sk)-[a-z0-9@.-]+ AAAA[A-Za-z0-9+/]*={0,2}( |$)`)

// checkOnlyGeneratedMaterial đỏ khi ví dụ chứa khoá, vân tay hay chữ ký mà bộ
// sinh không tạo ra: khoá lạ thì không ai có khoá riêng để ký lại, chữ ký lạ thì
// không được cập nhật khi đầu vào của nó đổi.
func checkOnlyGeneratedMaterial(t *testing.T, f exampleSet) {
	t.Helper()
	keys, fps, sigs := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, k := range testKeys {
		line, fp, err := k.publicKey()
		if err != nil {
			t.Fatal(err)
		}
		keys[line], fps[fp] = true, true
	}
	for _, s := range signedValues {
		sig, err := f.str(s.at)
		if err != nil {
			t.Fatal(err)
		}
		sigs[sig] = true
	}
	for _, name := range slices.Sorted(maps.Keys(f)) {
		var doc any
		if err := json.Unmarshal(f[name], &doc); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		visitStrings(doc, "", func(field, s string) {
			switch {
			case sshKeyLine.MatchString(s) && !keys[authorizedKey(s)]:
				t.Errorf("%s: khoá %q không suy ra từ hạt giống nào trong testKeys", name, s)
			case strings.HasPrefix(s, "SHA256:") && !fps[s]:
				t.Errorf("%s: vân tay %s không phải của khoá thử nghiệm nào", name, s)
			case field == "sig" && !sigs[s]:
				t.Errorf("%s: chữ ký %q không do bộ sinh tạo (thêm vào signedValues)", name, s)
			}
		})
	}
}

// visitStrings gọi fn với mọi chuỗi trong v, kèm tên trường gần nhất chứa nó.
func visitStrings(v any, field string, fn func(field, s string)) {
	switch x := v.(type) {
	case string:
		fn(field, x)
	case map[string]any:
		for k, e := range x {
			visitStrings(e, k, fn)
		}
	case []any:
		for _, e := range x {
			visitStrings(e, field, fn)
		}
	}
}
