# control panel to cast and analyze adversarial images

- you can find a couple of articles about adversarial immages at [aker](https://github.com/Belobragin/paker)  repositary in the `./ref` folder


## Overview
1. this is a set of API to process images with adversarial attacks and further use Resnet50 to analyze them. This is a part of integral `werewolf - aker - cr-resnet project`.
2. This project is for fun and personal use only and it is not a commercial product.
3. Code is served "as-is", with absolutely no gurantees or support, on the best effort basis. I will try to answer the questions, but can not promise that
4. this code was prepared with the assistance of Claude Sonnet 5.5


## Control panel code realization
This project is served as a local and an aws cloud options. Details are below.


## License and terms of use
This application is Apache licensed (no license modification). For citing use the next information:
```
Vladimir Belobragin
e-mail: vblvbl133@gmail.com
linkedin: https://www.linkedin.com/in/belovl%D0%B0d/?isSelfProfile=true
```

## Security
The server is cast on [gorilla/mux](https://github.com/gorilla/mux) router, and as on Aug.2026 this go package last update was more then 2 years ago. So, it is NOT secure and can not be recommended for production use or any public internet exposure. Run it locally behind the firewall and other security devices OR/AND, if run in the aws cloud with supplied tempates, use an ssh tunnel:
```
ssh -i [local path to your aws key] -N \
  -o ExitOnForwardFailure=yes -o ServerAliveInterval=30 \
  -L 8101:localhost:8101 ec2-user@[your ec2 instance public address]
```
No responsibility for this product use consequences. Play with the code on your own risk.

## More verbose product description

### course of fun:
- clone repos. For the cloud version prepare stacks with the attached aws cloudformation templates (below). For the local run, must inline with prerequsites (below)
- take any picture => upload to storage with `/werewolf/picture/upload` api => cast the adversarial image with `/werewolf/alter/{objectID}` api => diagnose the adversarial image with Resnet50 with `` api. Other apis are for convinience
- do not upload or download images from project storage directly - use `/werewolf/picture/upload` and `/werewolf/picture/download/{objectID}` api
- three independent adversarial algorythms are available for casting fake images
- most of the commands are described in the example makefile and makefile-aws
- two independent sets of environment variables must be used for the local and cloud versions - follow them in the files `.env.local.example` and `.env.awscloud.example`. Add your values to these files and remove`.example` extension from the names


### technical restrictions and limitations
1. image file dimension must be less then set by environment variable `MAX_IMAGE_SIZE` (for the local run) 
2. initial prognosis for ImageNet classes for adversarials will differ from classification by Resnet50. This divergence is small and intentional
3. this code for the local run was built and tested for Ubuntu 22. No way to run it in Windows; or at least the major part of the code must be re-written with significant efforts.
4. this project code was not tested for Linux distributives, other  then Ubuntu22 


### local run
1. Prerequisites:
   - [docker-compose](https://docs.docker.com/compose/)
   - [goose](https://github.com/pressly/goose) migration tool
   - [swag](https://pkg.go.dev/github.com/swaggo/swag/v2#section-readme) utility
   - [postgres](https://www.postgresql.org/) database with admin access
   - [nats server](https://nats.io/)
   - [resnet50](https://www.sciencedirect.com/topics/computer-science/resnet50) images classification neural net    
Attention: `nats server` and `resnet50` neural net can be easily obtained in containers with the attached code, if you have a pre-installed `docker-compose`
2. clone this code with:
```
git clone https://github.com/Belobragin/werewolf
```
put your values for the environment variables in the file `.env.local.example` and save it as `.env.local`  
3. to start nats server, run in a seperate terminal `make  startnats`, to stop run `make stopnats` On first run it will pull `nats server` image and assemble local image from containerfile, which can take a while - be ready to wait for 10-15 minutes
4. prepare `aker` and `crresnet` containers - follow guidlines in their repos  
5. make postgres database `werewolf` with the user `wf` and run required migrations `make lmigrate`. The project uses schema `wfn` which will be created automatically on migration process
6. run control panel server with `make lrun`. It is run in the foreground, so stop it with a `ctrl-c` keys  
7. local metrics  
- this projects outputs standard and custom metrics
- set environment variabl `METRICS_PORT` for metrics http server port
- obtain standard metrics on path `localhost:$METRICS_PORT/metrics/standard`
- obtain custom metrics on path `localhost:$METRICS_PORT/metrics/custom`


### aws cloud run
- if you have an aws account, you can run this project in the aws cloud. Please, be informed that it is NOT FREE and aws infrastructure can be expensive. Your expenses for this project may vary greatly.

If you decide to try this project in the aws cloud, follow next steps:
1. fill in the values for the cloud environment variables in the file `.env.awscloud.example` and save it as `.env.awscloud`.
2. run `make setcfresource`. Your aws credentials must be locally exported and allow the creation of all resources in the `iac/awscloud/template/WerewolfResourceTemplate.yaml` template
3. after stack is created, get ec2 external IP and put it to the variable `EC2_EXTERNAL_IP`, put the path to the private key for ec2 access to the variable `PATH_TO_AWS_KEY` in the `makefile-aws`, then in the seperate terminal window run `make tunnel`
4. in the browser window go to the page `localhost:8101/api/v1/werewolf/docs/swagger` and use swagger api UI
5. recommendations: do not forget to stop ec2 instance in the stack after you finish working with api. Or just delete the whole stack. 
 