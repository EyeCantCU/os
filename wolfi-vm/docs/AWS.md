# AWS Publishing docs

The awspublishing uses [awspub](https://canonical-awspub.readthedocs-hosted.com/en/latest/)

The configuration files for production publication are in `awspub/`
directory. This is used by `make prodawspub` target. It publishes AMI
into produciton marketplace account, does sharing it to all chainguard
Dev accounts, and also submits a version update to the marketplace
listings.

The configuration files for development publication are generated on
the fly. This is used by `make awspub` target. It publishes an AMI
into account, can optionally share it.

## Prod configs updates

### To change marketplace listing allow list

1. login to kim_chainguard in go/aws
2. Go to [server listings](https://aws.amazon.com/marketplace/management/products/server?#)
3. Select a listing, and add aws account id to allow list
4. This will enable those account to see the chainguard VMs AMI listings in the marketplace, request free trial, request private offer, and also see that they might have a private offer to accept

### To issue marketplace listing private offer

1. login to kim_chainguard in go/aws
2. Go to [private offers](https://aws.amazon.com/marketplace/management/offers) generate a new private offer
3. Offers are on per-listing basis, but can be for multiple related AWS accounts (most customers will have multiple AWS accounts)
4. Once offer is issued, the marketplace listing for said customers will look very nice and have blue banners saying hey you have a private offer to accept, or you already have access to this

There is currently no automation to bulk issue an offer, for multiple
VMs for both arches to a set of accounts. But it should be possible in
the future to create a workflow that would be able to do that. Or like
an aws-cli script.

### To perform direct AMI share
1. Update awspub/ configs
2. In share, add AWS account id, or AWS org arn, or AWS OU arn
3. Merge the pr
4. subsequent publications will share newly built AMIs with the update list of receipients.
5. If possible preffer marketplace route, as AMIs there are from a verified publisher and mirrored to all regions

### To update direct AMI region publication
1. Update awspub/ configs
2. Add additional regions to copy the AMIs to
3. Potentially regions may need to be activated on the kim_chainguard account, and allow in the organization in chaingaurd-dev/aws-infrastructure

### To publish to GovCloud
1. GovCloud is a separate AWS partition (everything is separate, as if a different AWS clone)
2. So far xnox had no credentials or access to GovCloud to see if our VMs work there, or if we can publish there
3. Also it is not certain if our account is verified to publish to GovCloud

### To change Marketplace listing details
1. Half of the listing details are editable by requesting changes from the [server products](https://aws.amazon.com/marketplace/management/products/server) page
2. Stuff that comes from AMI description, or Marketplace version are updatable in `awspub/`
