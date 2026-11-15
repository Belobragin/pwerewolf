package cloud

import (
	"context"
	"errors"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/belobragin/werewolf/bus/internal/pdata"
	"github.com/belobragin/werewolf/bus/pkg/filter"
	"github.com/belobragin/werewolf/bus/pkg/model/alter_model"
	"github.com/belobragin/werewolf/bus/pkg/model/diagnose_model"
	"github.com/belobragin/werewolf/bus/pkg/model/picture_model"
	"github.com/google/uuid"
)

// memStore is the table in memory. Every written item goes through the attributevalue marshaling,
// so a wrong attribute tag shows up here.
type memStore struct{ rows map[key]item }

func newMemStore() *memStore { return &memStore{rows: map[key]item{}} }

func (m *memStore) pick(ok func(item) bool) []item {
	var out []item
	for _, it := range m.rows {
		if ok(it) {
			out = append(out, it)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].WerewolfSortKey < out[j].WerewolfSortKey })
	return out
}

func (m *memStore) byUUID(_ context.Context, id string) ([]item, error) {
	return m.pick(func(it item) bool { return it.ImageObjectUUID == id }), nil
}

func (m *memStore) byName(_ context.Context, name string, entitiesOnly bool) ([]item, error) {
	return m.pick(func(it item) bool {
		return it.ImageObjectName == name && (!entitiesOnly || strings.HasPrefix(it.WerewolfSortKey, entitySortPrefix))
	}), nil
}

func (m *memStore) scanEntities(_ context.Context, f scanFilter) ([]item, error) {
	return m.pick(func(it item) bool {
		return strings.HasPrefix(it.WerewolfSortKey, entitySortPrefix) &&
			(f.ImageUUID == "" || it.ImageUUID == f.ImageUUID) &&
			(len(f.AdversarialUUIDs) == 0 || contains(f.AdversarialUUIDs, it.AdversarialUUID))
	}), nil
}

func contains(list []string, s string) bool {
	for _, e := range list {
		if e == s {
			return true
		}
	}
	return false
}

// write is a transaction: all or nothing, one operation per key, a conditional put fails on an existing key.
func (m *memStore) write(_ context.Context, w *writeSet) error {
	for k := range w.puts {
		if _, ok := w.dels[k]; ok {
			return errors.New("transaction has two operations on one item")
		}
		if _, exists := m.rows[k]; exists && w.puts[k].mustBeNew {
			return errConflict
		}
	}
	for k, p := range w.puts {
		av, err := attributevalue.MarshalMap(p.it)
		if err != nil {
			return err
		}
		var back item
		if err = attributevalue.UnmarshalMap(av, &back); err != nil {
			return err
		}
		m.rows[k] = back
	}
	for k := range w.dels {
		delete(m.rows, k)
	}
	return nil
}

func newMemDb() (*DbRealizationCloud, *memStore) {
	ms := newMemStore()
	return newCloudDb(ms), ms
}

func prediction(label string) diagnose_model.ResnetPredictionData {
	return diagnose_model.ResnetPredictionData{ClassLabel: label, ClassName: label, ClassID: 1, Probability: 12.5}
}

func predict(n int) []map[string]diagnose_model.ResnetPredictionData {
	out := make([]map[string]diagnose_model.ResnetPredictionData, n)
	for i := range out {
		out[i] = map[string]diagnose_model.ResnetPredictionData{"n" + string(rune('0'+i)): prediction("c" + string(rune('0'+i)))}
	}
	return out
}

func upload(t *testing.T, d *DbRealizationCloud, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	err := d.DbUploadImage(context.Background(), &picture_model.UploadReply{
		ImageID: id, ImageName: name, ImageDescription: "d", MimeType: "image/jpeg", ImageSize: "1Kb", ImageHeight: 2, ImageWidth: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func alterData(img uuid.UUID, advName, algo, goal string, withFilter bool, p []map[string]diagnose_model.ResnetPredictionData) *alter_model.AlterPostToDb {
	m := &alter_model.AlterPostToDb{
		ImageID: img, AdversarialObjectID: uuid.New(), AdversarialName: advName, Algorythm: algo, AlterGoal: goal, Predict: p,
	}
	if withFilter {
		m.FilterObjectID, m.FilterName = uuid.New(), strings.TrimSuffix(advName, ".jpg")+"_filter.jpg"
	}
	return m
}

func sortKeysOf(ms *memStore, name string) []string {
	var out []string
	for k := range ms.rows {
		if k.name == name {
			out = append(out, k.sortKey)
		}
	}
	sort.Strings(out)
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSortKey(t *testing.T) {
	for _, tc := range []struct {
		got, want string
	}{
		{sortKey(0, "fgsm", "tiger_shark"), "0_fgsm_tiger_shark"},
		{sortKey(2, "emulator", ""), "2_emulator_"},
		{sortKey(1, "", ""), "1__"},
		{entitySortKey("", ""), "__"},
		{entitySortKey("fgsm", "goal"), "_fgsm_goal"},
		{entitySortKey("fgsm", ""), "_fgsm_"},
	} {
		if tc.got != tc.want {
			t.Errorf("sort key %q, want %q", tc.got, tc.want)
		}
	}
}

func TestWildcardMatch(t *testing.T) {
	for _, tc := range []struct {
		pattern, s string
		want       bool
	}{
		{"hog.jpg", "hog.jpg", true},
		{"hog.jpg", "hog.png", false},
		{"*", "", true},
		{"*_adv_*", "hog_adv_fgsm.jpg", true},
		{"h?g*", "hog.jpg", true},
		{"h?g", "hg", false},
		{"a*b*c", "aXbYbZc", true},
		{"a*b*c", "aXbYbZ", false},
		{"100%_x", "100%_x", true},
		{"100%_x", "100ab_x", false},
		{"*.jpg", "hog.jpg.png", false},
	} {
		if got := wildcardMatch(tc.pattern, tc.s); got != tc.want {
			t.Errorf("wildcardMatch(%q, %q) = %v, want %v", tc.pattern, tc.s, got, tc.want)
		}
	}
}

// the attribute names are the column names of the wfn tables, the result column is diagnose_result.
func TestItemAttributes(t *testing.T) {
	rank := 0
	pred := item{
		ImageObjectName: "a.jpg", WerewolfSortKey: "0__", ImageObjectUUID: "u",
		PredictionRank: &rank, DiagnoseResult: map[string]diagnose_model.ResnetPredictionData{"n0": prediction("x")},
	}
	av, err := attributevalue.MarshalMap(pred)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"image_object_name", "werewolf_sort_key", "image_object_uuid", "prediction_rank", "diagnose_result"} {
		if _, ok := av[name]; !ok {
			t.Errorf("attribute %s is missing: %v", name, av)
		}
	}
	if n, ok := av["prediction_rank"].(*types.AttributeValueMemberN); !ok || n.Value != "0" {
		t.Errorf("rank 0 must be stored, got %v", av["prediction_rank"])
	}
	if _, ok := av["result"]; ok {
		t.Error("the column is diagnose_result, not result")
	}

	adv := newAdversarialItem(alterData(uuid.New(), "a_adv.jpg", "fgsm", "goal", true, nil), "t")
	av, _ = attributevalue.MarshalMap(adv)
	for _, name := range []string{"adversarial_object_uuid", "adversarial_object_name", "image_uuid", "with_filter_object", "algorythm", "alter_goal", "created_at"} {
		if _, ok := av[name]; !ok {
			t.Errorf("adversarial attribute %s is missing", name)
		}
	}
	if _, ok := av["prediction_rank"]; ok {
		t.Error("an entity row has no rank")
	}
}

func TestUploadAndLookup(t *testing.T) {
	ctx := context.Background()
	d, ms := newMemDb()
	img := upload(t, d, "hog.jpg")

	if k := (key{"hog.jpg", "__"}); ms.rows[k].ImageUUID != img.String() || ms.rows[k].ImageObjectUUID != img.String() {
		t.Errorf("stored rows: %v", ms.rows)
	}
	if err := d.DbUploadImage(ctx, &picture_model.UploadReply{ImageID: img, ImageName: "other.jpg"}); !errors.Is(err, pdata.ErrExistObjectID) {
		t.Errorf("same uuid: %v", err)
	}
	if err := d.DbUploadImage(ctx, &picture_model.UploadReply{ImageID: uuid.New(), ImageName: "hog.jpg"}); !errors.Is(err, pdata.ErrExistObjectName) {
		t.Errorf("same name: %v", err)
	}
	if err := d.DbUploadImage(ctx, nil); err == nil {
		t.Error("nil data is accepted")
	}

	if name, err := d.DbGetImageName(ctx, img); err != nil || name != "hog.jpg" {
		t.Errorf("image name: %q, %v", name, err)
	}
	if _, err := d.DbGetImageName(ctx, uuid.New()); !errors.Is(err, pdata.ErrNoObjectId) {
		t.Errorf("unknown uuid: %v", err)
	}
	if id, err := d.DbGetImageUuidByName(ctx, "hog.jpg", "image"); err != nil || id != img {
		t.Errorf("uuid by name: %v, %v", id, err)
	}
	if _, err := d.DbGetImageUuidByName(ctx, "hog.jpg", "adversarial"); !errors.Is(err, pdata.ErrNoObjectName) {
		t.Errorf("an image is not an adversarial object: %v", err)
	}
	if _, err := d.DbGetImageUuidByName(ctx, "none.jpg", "image"); !errors.Is(err, pdata.ErrNoObjectName) {
		t.Errorf("unknown name: %v", err)
	}
	if _, err := d.DbGetImageUuidByName(ctx, "hog.jpg", "video"); !errors.Is(err, pdata.ErrInvalidObjectKind) {
		t.Errorf("invalid kind: %v", err)
	}

	reply, err := d.DbDownloadImage(ctx, img)
	if err != nil || !reply.IfImage || reply.ObjectName != "hog.jpg" || reply.ObjectID != img || reply.CreatedAt == "" {
		t.Errorf("download reply: %+v, %v", reply, err)
	}
	if _, err = d.DbDownloadImage(ctx, uuid.New()); !errors.Is(err, pdata.ErrNoObjectId) {
		t.Errorf("unknown uuid: %v", err)
	}
}

func TestPostDiagnoseObject(t *testing.T) {
	ctx := context.Background()
	d, ms := newMemDb()
	img := upload(t, d, "hog.jpg")
	post := func(id uuid.UUID, n int) error {
		return d.DbPostDiagnoseObject(ctx, &diagnose_model.DiagnosePostReply{ImageID: id, ResnetReply: diagnose_model.ResnetReply{Predict: predict(n)}})
	}

	if _, err := d.DbGetDiagnoseObject(ctx, img); !errors.Is(err, pdata.ErrNoResultInDb) {
		t.Errorf("no predictions yet: %v", err)
	}
	if err := post(uuid.New(), 3); !errors.Is(err, pdata.ErrNoObjectId) {
		t.Errorf("unknown image: %v", err)
	}
	if err := post(img, 0); !errors.Is(err, diagnose_model.ErrEmptyPrediction) {
		t.Errorf("empty prediction: %v", err)
	}
	if err := d.DbPostDiagnoseObject(ctx, nil); err == nil {
		t.Error("nil data is accepted")
	}

	if err := post(img, 3); err != nil {
		t.Fatal(err)
	}
	if got := sortKeysOf(ms, "hog.jpg"); !equalStrings(got, []string{"0__", "1__", "2__", "__"}) {
		t.Errorf("sort keys: %v", got)
	}
	got, err := d.DbGetDiagnoseObject(ctx, img)
	if err != nil || got.ImageName != "hog.jpg" || got.ImageID != img || len(got.DbResultReply.Predict) != 3 {
		t.Fatalf("diagnose: %+v, %v", got, err)
	}
	for i, p := range got.DbResultReply.Predict {
		if _, ok := p[predictKey(i)]; !ok {
			t.Errorf("rank %d holds %v", i, p)
		}
	}

	// a repeated post replaces the ranks, keeps created_at of the ranks that stay and drops the surplus ones
	k := key{"hog.jpg", "0__"}
	old := ms.rows[k]
	old.CreatedAt = "2020-01-01T00:00:00.000000000Z"
	ms.rows[k] = old
	if err = post(img, 2); err != nil {
		t.Fatal(err)
	}
	if keys := sortKeysOf(ms, "hog.jpg"); !equalStrings(keys, []string{"0__", "1__", "__"}) {
		t.Errorf("sort keys after the repeat: %v", keys)
	}
	if ms.rows[k].CreatedAt != old.CreatedAt || ms.rows[k].UpdatedAt == "" || ms.rows[k].UpdatedAt <= old.CreatedAt {
		t.Errorf("rank 0 timestamps: %+v", ms.rows[k])
	}
	if got, _ = d.DbGetDiagnoseObject(ctx, img); got.DbResultReply.CreatedAt != old.CreatedAt {
		t.Errorf("reply created_at: %s", got.DbResultReply.CreatedAt)
	}
}

func predictKey(i int) string { return "n" + string(rune('0'+i)) }

func TestAlterObject(t *testing.T) {
	ctx := context.Background()
	d, ms := newMemDb()
	img := upload(t, d, "hog.jpg")

	first := alterData(img, "hog_adv.jpg", "fgsm", "goal", true, predict(3))
	if err := d.DbAlterObject(ctx, first, first.AdversarialObjectID); err != nil {
		t.Fatal(err)
	}
	if got := sortKeysOf(ms, "hog_adv.jpg"); !equalStrings(got, []string{"0_fgsm_goal", "1_fgsm_goal", "2_fgsm_goal", "_fgsm_goal"}) {
		t.Errorf("adversarial sort keys: %v", got)
	}
	adv := ms.rows[key{"hog_adv.jpg", "_fgsm_goal"}]
	if adv.ImageUUID != img.String() || adv.AdversarialObjectUUID != first.AdversarialObjectID.String() || !adv.WithFilterObject {
		t.Errorf("adversarial row: %+v", adv)
	}
	flt := ms.rows[key{"hog_adv_filter.jpg", "_fgsm_goal"}]
	if flt.AdversarialUUID != first.AdversarialObjectID.String() || flt.FilterObjectUUID != first.FilterObjectID.String() || flt.ImageUUID != "" {
		t.Errorf("filter row: %+v", flt)
	}
	if got := ms.rows[key{"hog_adv.jpg", "0_fgsm_goal"}]; got.AdversarialObjectUUID != first.AdversarialObjectID.String() || got.PredictionRank == nil {
		t.Errorf("prediction row: %+v", got)
	}
	if n := len(ms.rows[key{"hog.jpg", "0__"}].DiagnoseResult); n != 0 {
		t.Error("the image got the predictions of the adversarial object")
	}

	t.Run("the image of a filter is found through its adversarial object", func(t *testing.T) {
		if id, err := d.DbGetImageUuidByName(ctx, "hog_adv_filter.jpg", "filter"); err != nil || id != img {
			t.Errorf("filter: %v, %v", id, err)
		}
		if id, err := d.DbGetImageUuidByName(ctx, "hog_adv.jpg", "adversarial"); err != nil || id != img {
			t.Errorf("adversarial: %v, %v", id, err)
		}
	})
	t.Run("the adversarial data of the image", func(t *testing.T) {
		got, err := d.DbGetAdvDataImageId(ctx, img)
		if err != nil || len(got.AltergetAllreply) != 1 {
			t.Fatalf("got %+v, %v", got, err)
		}
		one := got.AltergetAllreply[0]
		if one.ImageID != img || one.AdversarialObjectID != first.AdversarialObjectID || one.FilterObjectID != first.FilterObjectID ||
			one.FilterName != "hog_adv_filter.jpg" || one.Algorythm != "fgsm" || one.AlterGoal != "goal" {
			t.Errorf("reply: %+v", one)
		}
		if _, err = d.DbGetAdvDataImageId(ctx, uuid.New()); !errors.Is(err, pdata.ErrNoObjectId) {
			t.Errorf("unknown image: %v", err)
		}
	})
	t.Run("the cascade list", func(t *testing.T) {
		got, err := d.DbListCascadeObjects(ctx, img)
		if err != nil || len(got) != 3 || got[0].Kind != "image" || got[1].Kind != "adversarial" || got[2].Kind != "filter" ||
			got[1].ID != first.AdversarialObjectID || got[2].ID != first.FilterObjectID {
			t.Errorf("image: %+v, %v", got, err)
		}
		got, err = d.DbListCascadeObjects(ctx, first.AdversarialObjectID)
		if err != nil || len(got) != 2 || got[0].Kind != "adversarial" || got[1].Kind != "filter" {
			t.Errorf("adversarial: %+v, %v", got, err)
		}
		got, err = d.DbListCascadeObjects(ctx, first.FilterObjectID)
		if err != nil || len(got) != 1 || got[0].Kind != "filter" {
			t.Errorf("filter: %+v, %v", got, err)
		}
		if _, err = d.DbListCascadeObjects(ctx, uuid.New()); !errors.Is(err, pdata.ErrNoObjectId) {
			t.Errorf("unknown uuid: %v", err)
		}
	})
	t.Run("an adversarial object and an image are not mixed up in the diagnose", func(t *testing.T) {
		if _, err := d.DbGetDiagnoseObject(ctx, first.AdversarialObjectID); !errors.Is(err, pdata.ErrNoObjectId) {
			t.Errorf("got %v", err)
		}
	})

	t.Run("a repeated result replaces the old objects with their filter and predictions", func(t *testing.T) {
		second := alterData(img, "hog_adv2.jpg", "fgsm", "", false, predict(2))
		if err := d.DbAlterObject(ctx, second, second.AdversarialObjectID); err != nil {
			t.Fatal(err)
		}
		if got := sortKeysOf(ms, "hog_adv.jpg"); len(got) != 0 {
			t.Errorf("old adversarial rows stay: %v", got)
		}
		if got := sortKeysOf(ms, "hog_adv_filter.jpg"); len(got) != 0 {
			t.Errorf("old filter rows stay: %v", got)
		}
		if got := sortKeysOf(ms, "hog_adv2.jpg"); !equalStrings(got, []string{"0_fgsm_", "1_fgsm_", "_fgsm_"}) {
			t.Errorf("an empty goal leaves the trailing underscore: %v", got)
		}
	})
	t.Run("the same result twice", func(t *testing.T) {
		same := alterData(img, "hog_adv3.jpg", "emulator", "", true, predict(1))
		for range 2 {
			if err := d.DbAlterObject(ctx, same, same.AdversarialObjectID); err != nil {
				t.Fatal(err)
			}
		}
		if got := sortKeysOf(ms, "hog_adv3.jpg"); !equalStrings(got, []string{"0_emulator_", "_emulator_"}) {
			t.Errorf("sort keys: %v", got)
		}
	})
	t.Run("the predictions of the image", func(t *testing.T) {
		data := alterData(img, "hog_adv4.jpg", "biterate_untargeted", "", false, predict(3))
		if err := d.DbAlterObject(ctx, data, img); err != nil {
			t.Fatal(err)
		}
		if got := sortKeysOf(ms, "hog.jpg"); !equalStrings(got, []string{"0__", "1__", "2__", "__"}) {
			t.Errorf("image sort keys: %v", got)
		}
		if got := sortKeysOf(ms, "hog_adv4.jpg"); !equalStrings(got, []string{"_biterate_untargeted_"}) {
			t.Errorf("adversarial sort keys: %v", got)
		}
	})
	t.Run("errors", func(t *testing.T) {
		data := alterData(uuid.New(), "x_adv.jpg", "fgsm", "", false, predict(1))
		if err := d.DbAlterObject(ctx, data, data.AdversarialObjectID); !errors.Is(err, pdata.ErrNoObjectId) {
			t.Errorf("unknown image: %v", err)
		}
		data = alterData(img, "y_adv.jpg", "fgsm", "", false, predict(1))
		if err := d.DbAlterObject(ctx, data, uuid.New()); !errors.Is(err, pdata.ErrNoObjectId) {
			t.Errorf("unknown owner of the predictions: %v", err)
		}
		if err := d.DbAlterObject(ctx, data, uuid.Nil); err == nil {
			t.Error("predictions without an owner are accepted")
		}
		if err := d.DbAlterObject(ctx, nil, uuid.Nil); err == nil {
			t.Error("nil data is accepted")
		}
		other := upload(t, d, "other.jpg")
		clash := alterData(other, "hog_adv3.jpg", "emulator", "", false, nil)
		if err := d.DbAlterObject(ctx, clash, uuid.Nil); !errors.Is(err, pdata.ErrExistObjectName) {
			t.Errorf("the name of another adversarial object: %v", err)
		}
	})
}

func TestDeleteImage(t *testing.T) {
	ctx := context.Background()
	d, ms := newMemDb()
	keep := upload(t, d, "keep.jpg")
	img := upload(t, d, "hog.jpg")
	fgsm := alterData(img, "hog_adv_fgsm.jpg", "fgsm", "g", true, predict(3))
	emu := alterData(img, "hog_adv_emu.jpg", "emulator", "", false, predict(2))
	for _, m := range []*alter_model.AlterPostToDb{fgsm, emu} {
		if err := d.DbAlterObject(ctx, m, m.AdversarialObjectID); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.DbPostDiagnoseObject(ctx, &diagnose_model.DiagnosePostReply{ImageID: img, ResnetReply: diagnose_model.ResnetReply{Predict: predict(3)}}); err != nil {
		t.Fatal(err)
	}
	if err := d.DbPostDiagnoseObject(ctx, &diagnose_model.DiagnosePostReply{ImageID: keep, ResnetReply: diagnose_model.ResnetReply{Predict: predict(1)}}); err != nil {
		t.Fatal(err)
	}

	t.Run("a filter alone", func(t *testing.T) {
		if err := d.DbDeleteImage(ctx, fgsm.FilterObjectID); err != nil {
			t.Fatal(err)
		}
		if got := sortKeysOf(ms, "hog_adv_fgsm_filter.jpg"); len(got) != 0 {
			t.Errorf("filter rows stay: %v", got)
		}
		if got := sortKeysOf(ms, "hog_adv_fgsm.jpg"); len(got) != 4 {
			t.Errorf("the adversarial rows must stay: %v", got)
		}
	})
	t.Run("an adversarial object with its predictions", func(t *testing.T) {
		if err := d.DbDeleteImage(ctx, fgsm.AdversarialObjectID); err != nil {
			t.Fatal(err)
		}
		if got := sortKeysOf(ms, "hog_adv_fgsm.jpg"); len(got) != 0 {
			t.Errorf("adversarial rows stay: %v", got)
		}
		if got := sortKeysOf(ms, "hog_adv_emu.jpg"); len(got) != 3 {
			t.Errorf("the other adversarial object must stay: %v", got)
		}
	})
	t.Run("an image with everything under it", func(t *testing.T) {
		if err := d.DbDeleteImage(ctx, img); err != nil {
			t.Fatal(err)
		}
		for k := range ms.rows {
			if k.name != "keep.jpg" {
				t.Errorf("row stays: %v", k)
			}
		}
		if got := sortKeysOf(ms, "keep.jpg"); !equalStrings(got, []string{"0__", "__"}) {
			t.Errorf("the other image: %v", got)
		}
	})
	if err := d.DbDeleteImage(ctx, uuid.New()); !errors.Is(err, pdata.ErrNoObjectId) {
		t.Errorf("unknown uuid: %v", err)
	}
}

func TestListImage(t *testing.T) {
	ctx := context.Background()
	d, _ := newMemDb()
	hog := upload(t, d, "hog.jpg")
	cat := upload(t, d, "cat.jpg")
	fgsm := alterData(hog, "hog_adv_fgsm.jpg", "fgsm", "wild_boar", true, nil)
	emu := alterData(hog, "hog_adv_emu.jpg", "emulator", "", false, nil)
	catAdv := alterData(cat, "cat_adv_fgsm.jpg", "fgsm", "", false, nil)
	for _, m := range []*alter_model.AlterPostToDb{fgsm, emu, catAdv} {
		if err := d.DbAlterObject(ctx, m, uuid.Nil); err != nil {
			t.Fatal(err)
		}
	}
	str := func(s string) *string { return &s }
	list := func(f filter.ImageFilter) []string {
		t.Helper()
		got, err := d.DbListImage(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, r := range got.AllImage {
			var parts []string
			for _, p := range []*string{r.ImageName, r.AdversarialName, r.FilterName} {
				if p != nil {
					parts = append(parts, *p)
				}
			}
			out = append(out, strings.Join(parts, "|"))
		}
		return out
	}

	if got := list(filter.ImageFilter{}); !equalStrings(got, []string{"hog.jpg", "cat.jpg"}) {
		t.Errorf("all images: %v", got)
	}
	if got := list(filter.ImageFilter{ImageName: str("c*")}); !equalStrings(got, []string{"cat.jpg"}) {
		t.Errorf("image name: %v", got)
	}
	if got := list(filter.ImageFilter{ImageID: &hog}); !equalStrings(got, []string{"hog.jpg"}) {
		t.Errorf("image uuid: %v", got)
	}
	if got := list(filter.ImageFilter{ImageName: str("")}); len(got) != 2 {
		t.Errorf("an empty name is not set: %v", got)
	}
	if got := list(filter.ImageFilter{Algorythm: str("fgsm")}); !equalStrings(got, []string{"hog.jpg|hog_adv_fgsm.jpg|hog_adv_fgsm_filter.jpg", "cat.jpg|cat_adv_fgsm.jpg"}) {
		t.Errorf("algorythm: %v", got)
	}
	if got := list(filter.ImageFilter{AdversarialName: str("*emu*")}); !equalStrings(got, []string{"hog.jpg|hog_adv_emu.jpg"}) {
		t.Errorf("adversarial name: %v", got)
	}
	if got := list(filter.ImageFilter{AlterGoal: str("wild_?oar")}); !equalStrings(got, []string{"hog.jpg|hog_adv_fgsm.jpg|hog_adv_fgsm_filter.jpg"}) {
		t.Errorf("alter goal: %v", got)
	}
	if got := list(filter.ImageFilter{AdversarialID: &emu.AdversarialObjectID}); !equalStrings(got, []string{"hog.jpg|hog_adv_emu.jpg"}) {
		t.Errorf("adversarial uuid: %v", got)
	}
	if got := list(filter.ImageFilter{FilterName: str("*filter*"), ImageName: str("hog*")}); !equalStrings(got, []string{"hog.jpg|hog_adv_fgsm.jpg|hog_adv_fgsm_filter.jpg"}) {
		t.Errorf("filter name: %v", got)
	}
	if got := list(filter.ImageFilter{FilterID: &fgsm.FilterObjectID, ImageName: str("cat*")}); len(got) != 0 {
		t.Errorf("filters do not match the other image: %v", got)
	}
	reply, _ := d.DbListImage(ctx, filter.ImageFilter{ImageName: str("none")})
	if reply.AllImage == nil || len(reply.AllImage) != 0 {
		t.Errorf("an empty list must not be nil: %+v", reply)
	}
}

// a recording fake of the dynamodb client
type fakeDdb struct {
	queries []*dynamodb.QueryInput
	scans   []*dynamodb.ScanInput
	txs     []*dynamodb.TransactWriteItemsInput
	pages   []map[string]types.AttributeValue // items of the scan pages
	txErr   error
}

func (f *fakeDdb) Query(_ context.Context, in *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	cp := *in
	f.queries = append(f.queries, &cp)
	return &dynamodb.QueryOutput{}, nil
}

func (f *fakeDdb) Scan(_ context.Context, in *dynamodb.ScanInput, _ ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	cp := *in
	f.scans = append(f.scans, &cp)
	page := len(f.scans) - 1
	out := &dynamodb.ScanOutput{}
	if page < len(f.pages) {
		out.Items = []map[string]types.AttributeValue{f.pages[page]}
	}
	if page+1 < len(f.pages) {
		out.LastEvaluatedKey = map[string]types.AttributeValue{"k": &types.AttributeValueMemberS{Value: "next"}}
	}
	return out, nil
}

func (f *fakeDdb) TransactWriteItems(_ context.Context, in *dynamodb.TransactWriteItemsInput, _ ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
	f.txs = append(f.txs, in)
	return &dynamodb.TransactWriteItemsOutput{}, f.txErr
}

func TestDdbStoreRequests(t *testing.T) {
	ctx := context.Background()
	f := &fakeDdb{}
	s := &ddbStore{api: f, table: "tbl"}

	if _, err := s.byUUID(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	if q := f.queries[0]; aws.ToString(q.TableName) != "tbl" || aws.ToString(q.IndexName) != "image_object_uuid" || aws.ToString(q.KeyConditionExpression) == "" {
		t.Errorf("byUUID query: %+v", q)
	}
	if _, err := s.byName(ctx, "a.jpg", true); err != nil {
		t.Fatal(err)
	}
	if q := f.queries[1]; q.IndexName != nil || !aws.ToBool(q.ConsistentRead) || !strings.Contains(aws.ToString(q.KeyConditionExpression), "begins_with") {
		t.Errorf("byName query: %+v", q)
	}
	if _, err := s.byName(ctx, "a.jpg", false); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(aws.ToString(f.queries[2].KeyConditionExpression), "begins_with") {
		t.Error("all rows of the partition are asked, no sort key condition expected")
	}
}

func TestDdbStoreScanPages(t *testing.T) {
	page := func(name string) map[string]types.AttributeValue {
		av, err := attributevalue.MarshalMap(item{ImageObjectName: name, WerewolfSortKey: "__", ImageObjectUUID: name})
		if err != nil {
			t.Fatal(err)
		}
		return av
	}
	f := &fakeDdb{pages: []map[string]types.AttributeValue{page("a"), page("b")}}
	s := &ddbStore{api: f, table: "tbl"}

	got, err := s.scanEntities(context.Background(), scanFilter{ImageUUID: "i1", AdversarialUUIDs: []string{"a1", "a2"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ImageObjectName != "a" || got[1].ImageObjectName != "b" || len(f.scans) != 2 {
		t.Errorf("rows %+v, scans %d", got, len(f.scans))
	}
	if f.scans[1].ExclusiveStartKey == nil {
		t.Error("the second page must start after the first")
	}
	expr := aws.ToString(f.scans[0].FilterExpression)
	if !strings.Contains(expr, "begins_with") || !strings.Contains(expr, "IN") || !strings.Contains(expr, " AND ") {
		t.Errorf("filter expression: %s", expr)
	}
}

func TestDdbStoreWrite(t *testing.T) {
	ctx := context.Background()
	f := &fakeDdb{}
	s := &ddbStore{api: f, table: "tbl"}

	w := newWriteSet()
	w.put(item{ImageObjectName: "new", WerewolfSortKey: "__"}, true)
	w.put(item{ImageObjectName: "plain", WerewolfSortKey: "__"}, false)
	if err := s.write(ctx, w); err != nil {
		t.Fatal(err)
	}
	conditional := 0
	for _, op := range f.txs[0].TransactItems {
		if op.Put.ConditionExpression != nil {
			conditional++
		}
	}
	if len(f.txs) != 1 || len(f.txs[0].TransactItems) != 2 || conditional != 1 {
		t.Errorf("transaction: %+v", f.txs)
	}

	f.txs = nil
	big := newWriteSet()
	for i := range 250 {
		big.del(key{"n" + string(rune('A'+i%26)) + strings.Repeat("x", i), "__"})
	}
	if err := s.write(ctx, big); err != nil {
		t.Fatal(err)
	}
	if len(f.txs) != 3 || len(f.txs[0].TransactItems) != 100 || len(f.txs[2].TransactItems) != 50 {
		t.Errorf("a transaction writes at most 100 items, got %d transactions", len(f.txs))
	}

	f.txErr = &types.TransactionCanceledException{CancellationReasons: []types.CancellationReason{
		{Code: aws.String("None")}, {Code: aws.String("ConditionalCheckFailed")},
	}}
	if err := s.write(ctx, w); !errors.Is(err, errConflict) {
		t.Errorf("a failed condition is a conflict: %v", err)
	}
	f.txErr = errors.New("boom")
	if err := s.write(ctx, w); err == nil || errors.Is(err, errConflict) {
		t.Errorf("other errors pass: %v", err)
	}
}

func TestWriteSetKeepsOneOperationPerKey(t *testing.T) {
	w := newWriteSet()
	k := key{"a", "__"}
	w.del(k)
	w.put(item{ImageObjectName: "a", WerewolfSortKey: "__"}, true)
	if len(w.dels) != 0 || w.puts[k].mustBeNew {
		t.Errorf("a put replaces the delete of its key and cannot be conditional: %+v", w)
	}
	w.del(k)
	if len(w.dels) != 0 {
		t.Error("a delete of a put key is ignored")
	}
}

type liveStack string

func (s liveStack) GetStackName() string { return string(s) }

// the live test writes temporary rows to the table of the stack and removes them; it is skipped without aws access.
// The index is eventually consistent, so the reads are repeated.
func TestCloudDbLive(t *testing.T) {
	stack := os.Getenv("AWS_RESOURCE_CFN_STACK_NAME")
	if stack == "" {
		t.Skip("AWS_RESOURCE_CFN_STACK_NAME is not set")
	}
	d, err := BuildDynamoDbS(liveStack(stack))
	if err != nil {
		t.Skipf("the dynamodb table is not available: %v", err)
	}
	ctx := context.Background()
	eventually := func(what string, check func() error) {
		t.Helper()
		var last error
		for range 30 {
			if last = check(); last == nil {
				return
			}
			time.Sleep(time.Second)
		}
		t.Fatalf("%s: %v", what, last)
	}

	name := "tmp-live-" + uuid.NewString() + ".jpg"
	img := upload(t, d, name)
	t.Cleanup(func() { _ = d.DbDeleteImage(ctx, img) })

	eventually("image name", func() error {
		got, err := d.DbGetImageName(ctx, img)
		if err == nil && got != name {
			err = errors.New("wrong name " + got)
		}
		return err
	})
	adv := alterData(img, strings.TrimSuffix(name, ".jpg")+"_adv.jpg", "fgsm", "goal", true, predict(3))
	if err = d.DbAlterObject(ctx, adv, adv.AdversarialObjectID); err != nil {
		t.Fatal(err)
	}
	eventually("adversarial data", func() error {
		got, err := d.DbGetAdvDataImageId(ctx, img)
		if err == nil && (len(got.AltergetAllreply) != 1 || got.AltergetAllreply[0].FilterName == "") {
			err = errors.New("unexpected reply")
		}
		return err
	})
	eventually("cascade list", func() error {
		got, err := d.DbListCascadeObjects(ctx, img)
		if err == nil && len(got) != 3 {
			err = errors.New("want image, adversarial object and filter")
		}
		return err
	})
	if err = d.DbDeleteImage(ctx, img); err != nil {
		t.Fatal(err)
	}
	eventually("deleted", func() error {
		if _, err := d.DbGetImageName(ctx, img); !errors.Is(err, pdata.ErrNoObjectId) {
			return errors.New("the image is still there")
		}
		return nil
	})
}
