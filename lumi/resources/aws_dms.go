package resources

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/databasemigrationservice"
	"github.com/rs/zerolog/log"
	"go.mondoo.io/mondoo/lumi/library/jobpool"
)

func (d *lumiAwsDms) id() (string, error) {
	return "aws.dms", nil
}

func (d *lumiAwsDms) GetReplicationInstances() ([]interface{}, error) {
	res := []interface{}{}
	poolOfJobs := jobpool.CreatePool(d.getReplicationInstances(), 5)
	poolOfJobs.Run()

	// check for errors
	if poolOfJobs.HasErrors() {
		return nil, poolOfJobs.GetErrors()
	}
	// get all the results
	for i := range poolOfJobs.Jobs {
		res = append(res, poolOfJobs.Jobs[i].Result.([]interface{})...)
	}

	return res, nil
}

func (d *lumiAwsDms) getReplicationInstances() []*jobpool.Job {
	var tasks = make([]*jobpool.Job, 0)
	at, err := awstransport(d.Runtime.Motor.Transport)
	if err != nil {
		return []*jobpool.Job{{Err: err}}
	}
	regions, err := at.GetRegions()
	if err != nil {
		return []*jobpool.Job{{Err: err}}
	}

	for _, region := range regions {
		regionVal := region
		f := func() (jobpool.JobResult, error) {
			log.Debug().Msgf("calling aws with region %s", regionVal)

			svc := at.Dms(regionVal)
			ctx := context.Background()
			replicationInstancesAggregated := []databasemigrationservice.ReplicationInstance{}

			var marker *string
			for {
				replicationInstances, err := svc.DescribeReplicationInstancesRequest(&databasemigrationservice.DescribeReplicationInstancesInput{Marker: marker}).Send(ctx)
				if err != nil {
					return nil, err
				}
				replicationInstancesAggregated = append(replicationInstancesAggregated, replicationInstances.ReplicationInstances...)

				if replicationInstances.Marker == nil {
					break
				}
				marker = replicationInstances.Marker
			}
			res, err := jsonToDictSlice(replicationInstancesAggregated)
			if err != nil {
				return nil, err
			}
			return jobpool.JobResult(res), nil
		}
		tasks = append(tasks, jobpool.NewJob(f))
	}
	return tasks
}