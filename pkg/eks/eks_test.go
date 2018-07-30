package eks_test

import (
	"time"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	cfn "github.com/aws/aws-sdk-go/service/cloudformation"
	awseks "github.com/aws/aws-sdk-go/service/eks"
	"github.com/kubicorn/kubicorn/pkg/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	. "github.com/weaveworks/eksctl/pkg/eks"
	"github.com/weaveworks/eksctl/pkg/testutils"
)

var _ = Describe("Eks", func() {
	var (
		c       *ClusterProvider
		p       *testutils.MockProvider
		output string
	)

	BeforeEach(func() {
		output = "log"
	})

	Describe("ListAll", func() {
		Context("With a cluster name", func() {
			var (
				clusterName string
				err         error
				created     time.Time
			)

			BeforeEach(func() {
				clusterName = "test-cluster"
				created = time.Now()

				p = testutils.NewMockProvider()

				c = &ClusterProvider{
					Spec: &ClusterConfig{
						ClusterName: clusterName,
					},
					Provider: p,
				}

				p.MockEKS().On("DescribeCluster", mock.MatchedBy(func(input *awseks.DescribeClusterInput) bool {
					return *input.Name == clusterName
				})).Return(&awseks.DescribeClusterOutput{
					Cluster: &awseks.Cluster{
						Name:      aws.String(clusterName),
						Status:    aws.String(awseks.ClusterStatusActive),
						Arn:       aws.String("arn-12345678"),
						CreatedAt: &created,
						ResourcesVpcConfig: &awseks.VpcConfigResponse{
							VpcId:     aws.String("vpc-1234"),
							SubnetIds: []*string{aws.String("sub1"), aws.String("sub2")},
						},
					},
				}, nil)
			})

			Context("and normal log level", func() {
				BeforeEach(func() {
					logger.Level = 3
				})

				JustBeforeEach(func() {
					err = c.ListClusters(100, output)
				})

				It("should not error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should have called AWS EKS service once", func() {
					Expect(p.MockEKS().AssertNumberOfCalls(GinkgoT(), "DescribeCluster", 1)).To(BeTrue())
				})

				It("should not call AWS CFN ListStackPages", func() {
					Expect(p.MockCloudFormation().AssertNumberOfCalls(GinkgoT(), "ListStacksPages", 0)).To(BeTrue())
				})
			})

			Context("and debug log level", func() {
				var (
					expectedStatusFilter string
				)
				BeforeEach(func() {
					expectedStatusFilter = "CREATE_COMPLETE"

					logger.Level = 4

					p.MockCloudFormation().On("ListStacksPages", mock.MatchedBy(func(input *cfn.ListStacksInput) bool {
						return *input.StackStatusFilter[0] == expectedStatusFilter
					}), mock.Anything).Return(nil)
				})

				JustBeforeEach(func() {
					err = c.ListClusters(100, output)
				})

				It("should not error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should have called AWS EKS service once", func() {
					Expect(p.MockEKS().AssertNumberOfCalls(GinkgoT(), "DescribeCluster", 1)).To(BeTrue())
				})

				It("should have called AWS CFN ListStackPages", func() {
					Expect(p.MockCloudFormation().AssertNumberOfCalls(GinkgoT(), "ListStacksPages", 1)).To(BeTrue())
				})
			})
		})

		Context("with no cluster name", func() {
			var (
				err         error
				chunkSize int
			)

			Context("and chunk-size of 1", func() {
				var (
					callNumber int
				)
				BeforeEach(func() {
					chunkSize = 1
					callNumber = 0

					p = testutils.NewMockProvider()
	
					c = &ClusterProvider{
						Spec: &ClusterConfig{},
						Provider: p,
					}

					mockResultFn := func(input *awseks.ListClustersInput) *awseks.ListClustersOutput {
						clusterName:= fmt.Sprintf("cluster-%d", callNumber)
						output := &awseks.ListClustersOutput{
							Clusters: []*string{aws.String(clusterName)},
						}
						if callNumber == 0 {
							output.NextToken = aws.String("SOMERANDOMTOKEN")
						}

						callNumber = callNumber + 1
						return output
					}
	
					p.MockEKS().On("ListClusters", mock.MatchedBy(func(input *awseks.ListClustersInput) bool {
						return *input.MaxResults == int64(chunkSize)
					})).Return(mockResultFn, nil)
				})

				JustBeforeEach(func() {
					err = c.ListClusters(chunkSize, output)
				})

				It("should not error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should have called AWS EKS service twice", func() {
					Expect(p.MockEKS().AssertNumberOfCalls(GinkgoT(), "ListClusters", 2)).To(BeTrue())
				})
			})
			Context("and chunk-size of 100", func() {
				BeforeEach(func() {
					chunkSize = 100

					p = testutils.NewMockProvider()
	
					c = &ClusterProvider{
						Spec: &ClusterConfig{},
						Provider: p,
					}

					mockResultFn := func(input *awseks.ListClustersInput) *awseks.ListClustersOutput {
						output := &awseks.ListClustersOutput{
							Clusters: []*string{aws.String("cluster-1"), aws.String("cluster-2")},
						}
						return output
					}
	
					p.MockEKS().On("ListClusters", mock.MatchedBy(func(input *awseks.ListClustersInput) bool {
						return *input.MaxResults == int64(chunkSize)
					})).Return(mockResultFn, nil)
				})

				JustBeforeEach(func() {
					err = c.ListClusters(chunkSize, output)
				})

				It("should not error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should have called AWS EKS service once", func() {
					Expect(p.MockEKS().AssertNumberOfCalls(GinkgoT(), "ListClusters", 1)).To(BeTrue())
				})
			})
		})
		
	})
})
