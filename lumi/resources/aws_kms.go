package resources

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/rs/zerolog/log"
	"go.mondoo.io/mondoo/lumi"
	"go.mondoo.io/mondoo/lumi/library/jobpool"
)

func (k *lumiAwsKms) id() (string, error) {
	return "aws.kms", nil
}

func (k *lumiAwsKms) GetKeys() ([]interface{}, error) {
	res := []interface{}{}
	poolOfJobs := jobpool.CreatePool(k.getKeys(), 5)
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

func (k *lumiAwsKms) getKeys() []*jobpool.Job {
	var tasks = make([]*jobpool.Job, 0)
	at, err := awstransport(k.Runtime.Motor.Transport)
	if err != nil {
		return []*jobpool.Job{&jobpool.Job{Err: err}}
	}
	regions, err := at.GetRegions()
	if err != nil {
		return []*jobpool.Job{&jobpool.Job{Err: err}}
	}

	for _, region := range regions {
		regionVal := region
		f := func() (jobpool.JobResult, error) {
			log.Debug().Msgf("calling aws with region %s", regionVal)

			svc := at.Kms(regionVal)
			ctx := context.Background()
			res := []interface{}{}
			var marker *string
			for {
				keyList, err := svc.ListKeysRequest(&kms.ListKeysInput{Marker: marker}).Send(ctx)
				if err != nil {
					return nil, err
				}

				for _, key := range keyList.Keys {
					lumiRecorder, err := k.Runtime.CreateResource("aws.kms.key",
						"id", toString(key.KeyId),
						"arn", toString(key.KeyArn),
						"region", regionVal,
					)
					if err != nil {
						return nil, err
					}
					res = append(res, lumiRecorder)
				}
				if keyList.Truncated == nil || *keyList.Truncated == false {
					break
				}
				marker = keyList.NextMarker
			}
			return jobpool.JobResult(res), nil
		}
		tasks = append(tasks, jobpool.NewJob(f))
	}
	return tasks
}

func (k *lumiAwsKmsKey) GetMetadata() (interface{}, error) {
	key, err := k.Arn()
	if err != nil {
		return nil, err
	}
	region, err := k.Region()
	if err != nil {
		return nil, err
	}
	at, err := awstransport(k.Runtime.Motor.Transport)
	if err != nil {
		return nil, err
	}
	svc := at.Kms(region)
	ctx := context.Background()

	keyMetadata, err := svc.DescribeKeyRequest(&kms.DescribeKeyInput{KeyId: &key}).Send(ctx)
	if err != nil {
		return nil, err
	}
	return jsonToDict(keyMetadata.DescribeKeyOutput.KeyMetadata)
}

func (k *lumiAwsKmsKey) GetKeyRotationEnabled() (bool, error) {
	keyId, err := k.Id()
	if err != nil {
		return false, err
	}
	region, err := k.Region()
	if err != nil {
		return false, err
	}
	at, err := awstransport(k.Runtime.Motor.Transport)
	if err != nil {
		return false, err
	}
	svc := at.Kms(region)
	ctx := context.Background()

	key, err := svc.GetKeyRotationStatusRequest(&kms.GetKeyRotationStatusInput{KeyId: &keyId}).Send(ctx)
	if err != nil {
		return false, err
	}
	return toBool(key.KeyRotationEnabled), nil
}

func (k *lumiAwsKmsKey) id() (string, error) {
	return k.Arn()
}

func (p *lumiAwsKmsKey) init(args *lumi.Args) (*lumi.Args, AwsKmsKey, error) {
	if len(*args) > 2 {
		return args, nil, nil
	}

	if (*args)["arn"] == nil {
		return nil, nil, errors.New("arn required to fetch aws kms key")
	}

	// load all keys
	obj, err := p.Runtime.CreateResource("aws.kms")
	if err != nil {
		return nil, nil, err
	}
	aws := obj.(AwsKms)

	rawResources, err := aws.Keys()
	if err != nil {
		return nil, nil, err
	}

	arnVal := (*args)["arn"].(string)
	for i := range rawResources {
		key := rawResources[i].(AwsKmsKey)
		lumiKeyArn, err := key.Arn()
		if err != nil {
			return nil, nil, errors.New("kms key does not exist")
		}
		if lumiKeyArn == arnVal {
			return args, key, nil
		}
	}
	return nil, nil, errors.New("kms key does not exist")
}