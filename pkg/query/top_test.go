package query

import (
	"context"
	"os"
	"testing"

	"github.com/go-kit/log"
	"github.com/google/pprof/profile"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"

	metastorev1alpha1 "github.com/parca-dev/parca/gen/proto/go/parca/metastore/v1alpha1"
	pb "github.com/parca-dev/parca/gen/proto/go/parca/query/v1alpha1"
	"github.com/parca-dev/parca/pkg/metastore"
	parcaprofile "github.com/parca-dev/parca/pkg/profile"
)

func TestGenerateTopTable(t *testing.T) {
	ctx := context.Background()

	f, err := os.Open("testdata/alloc_objects.pb.gz")
	require.NoError(t, err)
	p1, err := profile.Parse(f)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	l := metastore.NewBadgerMetastore(
		log.NewNopLogger(),
		prometheus.NewRegistry(),
		trace.NewNoopTracerProvider().Tracer(""),
		metastore.NewRandomUUIDGenerator(),
	)
	t.Cleanup(func() {
		l.Close()
	})
	p, err := parcaprofile.FlatProfileFromPprof(ctx, log.NewNopLogger(), l, p1, 0)
	require.NoError(t, err)

	res, err := GenerateTopTable(ctx, l, p)
	require.NoError(t, err)

	require.Equal(t, int32(1886), res.Total)
	require.Equal(t, int32(899), res.Reported)
	require.Len(t, res.List, 899)

	found := false
	for _, node := range res.GetList() {
		if node.GetMeta().GetMapping().GetFile() == "/bin/operator" &&
			node.GetMeta().GetFunction().GetName() == "encoding/json.Unmarshal" {
			require.Equal(t, int64(3135531), node.GetCumulative())
			// TODO(metalmatze): This isn't fully deterministic yet, thus some assertions are commented.
			// require.Equal(t, int64(32773), node.GetFlat())

			// require.Equal(t, uint64(7578336), node.GetMeta().GetLocation().GetAddress())
			require.Equal(t, false, node.GetMeta().GetLocation().GetIsFolded())
			require.Equal(t, uint64(4194304), node.GetMeta().GetMapping().GetStart())
			require.Equal(t, uint64(23252992), node.GetMeta().GetMapping().GetLimit())
			require.Equal(t, uint64(0), node.GetMeta().GetMapping().GetOffset())
			require.Equal(t, "/bin/operator", node.GetMeta().GetMapping().GetFile())
			require.Equal(t, "", node.GetMeta().GetMapping().GetBuildId())
			require.Equal(t, true, node.GetMeta().GetMapping().GetHasFunctions())
			require.Equal(t, false, node.GetMeta().GetMapping().GetHasFilenames())
			require.Equal(t, false, node.GetMeta().GetMapping().GetHasLineNumbers())
			require.Equal(t, false, node.GetMeta().GetMapping().GetHasInlineFrames())

			require.Equal(t, int64(0), node.GetMeta().GetFunction().GetStartLine())
			require.Equal(t, "encoding/json.Unmarshal", node.GetMeta().GetFunction().GetName())
			require.Equal(t, "encoding/json.Unmarshal", node.GetMeta().GetFunction().GetSystemName())
			// require.Equal(t, int64(101), node.GetMeta().GetLine().GetLine())
			found = true
			break
		}
	}
	require.Truef(t, found, "expected to find the specific function")
}

func TestAggregateTopByFunction(t *testing.T) {
	uuid1 := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	uuid2 := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2}
	uuid3 := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 3}

	testcases := []struct {
		name   string
		input  *pb.Top
		output *pb.Top
	}{{
		name: "Empty",
		input: &pb.Top{
			List: []*pb.TopNode{},
		},
		output: &pb.Top{
			List: []*pb.TopNode{},
		},
	}, {
		name: "NoMeta",
		input: &pb.Top{
			List: []*pb.TopNode{
				{Meta: nil, Cumulative: 1, Flat: 1},
				{Meta: nil, Cumulative: 2, Flat: 2},
			},
		},
		output: &pb.Top{
			List: []*pb.TopNode{},
		},
	}, {
		name: "UniqueAddress",
		input: &pb.Top{
			List: []*pb.TopNode{
				{
					Meta: &pb.TopNodeMeta{
						Mapping:  &metastorev1alpha1.Mapping{Id: uuid1},
						Location: &metastorev1alpha1.Location{Id: uuid2, Address: 2},
					},
					Cumulative: 1,
					Flat:       1,
				},
				{
					Meta: &pb.TopNodeMeta{
						Mapping:  &metastorev1alpha1.Mapping{Id: uuid1},
						Location: &metastorev1alpha1.Location{Id: uuid3, Address: 3},
					},
					Cumulative: 1,
					Flat:       1,
				},
			},
		},
		output: &pb.Top{
			List: []*pb.TopNode{
				{
					Meta: &pb.TopNodeMeta{
						Mapping:  &metastorev1alpha1.Mapping{Id: uuid1},
						Location: &metastorev1alpha1.Location{Id: uuid2, Address: 2},
					},
					Cumulative: 1,
					Flat:       1,
				},
				{
					Meta: &pb.TopNodeMeta{
						Mapping:  &metastorev1alpha1.Mapping{Id: uuid1},
						Location: &metastorev1alpha1.Location{Id: uuid3, Address: 3},
					},
					Cumulative: 1,
					Flat:       1,
				},
			},
		},
	}, {
		name: "UniqueFunction",
		input: &pb.Top{
			List: []*pb.TopNode{
				{
					Meta: &pb.TopNodeMeta{
						Mapping:  &metastorev1alpha1.Mapping{Id: uuid1},
						Location: &metastorev1alpha1.Location{Id: uuid2, Address: 2},
						Function: &metastorev1alpha1.Function{Id: uuid2, Name: "func2"},
					},
					Cumulative: 1,
					Flat:       1,
				},
				{
					Meta: &pb.TopNodeMeta{
						Mapping:  &metastorev1alpha1.Mapping{Id: uuid1},
						Location: &metastorev1alpha1.Location{Id: uuid3, Address: 3},
						Function: &metastorev1alpha1.Function{Id: uuid3, Name: "func3"},
					},
					Cumulative: 1,
					Flat:       1,
				},
			},
		},
		output: &pb.Top{
			List: []*pb.TopNode{
				{
					Meta: &pb.TopNodeMeta{
						Mapping:  &metastorev1alpha1.Mapping{Id: uuid1},
						Location: &metastorev1alpha1.Location{Id: uuid2, Address: 2},
						Function: &metastorev1alpha1.Function{Id: uuid2, Name: "func2"},
					},
					Cumulative: 1,
					Flat:       1,
				},
				{
					Meta: &pb.TopNodeMeta{
						Mapping:  &metastorev1alpha1.Mapping{Id: uuid1},
						Location: &metastorev1alpha1.Location{Id: uuid3, Address: 3},
						Function: &metastorev1alpha1.Function{Id: uuid3, Name: "func3"},
					},
					Cumulative: 1,
					Flat:       1,
				},
			},
		},
	}, {
		name: "AggregateAddress",
		input: &pb.Top{
			List: []*pb.TopNode{
				{
					Meta: &pb.TopNodeMeta{
						Mapping:  &metastorev1alpha1.Mapping{Id: uuid1},
						Location: &metastorev1alpha1.Location{Id: uuid2, Address: 2},
					},
					Cumulative: 1,
					Flat:       1,
				},
				{
					Meta: &pb.TopNodeMeta{
						Mapping:  &metastorev1alpha1.Mapping{Id: uuid1},
						Location: &metastorev1alpha1.Location{Id: uuid2, Address: 2},
					},
					Cumulative: 1,
					Flat:       1,
				},
			},
		},
		output: &pb.Top{
			List: []*pb.TopNode{
				{
					Meta: &pb.TopNodeMeta{
						Mapping:  &metastorev1alpha1.Mapping{Id: uuid1},
						Location: &metastorev1alpha1.Location{Id: uuid2, Address: 2},
					},
					Cumulative: 2,
					Flat:       2,
				},
			},
		},
	}, {
		name: "AggregateFunction",
		input: &pb.Top{
			List: []*pb.TopNode{
				{
					Meta: &pb.TopNodeMeta{
						Mapping:  &metastorev1alpha1.Mapping{Id: uuid1},
						Location: &metastorev1alpha1.Location{Id: uuid2, Address: 2},
						Function: &metastorev1alpha1.Function{Id: uuid2, Name: "func2"},
					},
					Cumulative: 1,
					Flat:       1,
				},
				{
					Meta: &pb.TopNodeMeta{
						Mapping:  &metastorev1alpha1.Mapping{Id: uuid1},
						Location: &metastorev1alpha1.Location{Id: uuid2, Address: 2},
						Function: &metastorev1alpha1.Function{Id: uuid2, Name: "func2"},
					},
					Cumulative: 2,
					Flat:       2,
				},
			},
		},
		output: &pb.Top{
			List: []*pb.TopNode{
				{
					Meta: &pb.TopNodeMeta{
						Mapping:  &metastorev1alpha1.Mapping{Id: uuid1},
						Location: &metastorev1alpha1.Location{Id: uuid2, Address: 2},
						Function: &metastorev1alpha1.Function{Id: uuid2, Name: "func2"},
					},
					Cumulative: 3,
					Flat:       3,
				},
			},
		},
	}}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.output, aggregateTopByFunction(tc.input))
		})
	}
}
