<<<<<<< HEAD

=======
#!/bin/bash
>>>>>>> 01c153afcd4da61c184d4d53ed063f29695a52b0

set -a
source .env
set +a

<<<<<<< HEAD
echo dckr_pat_kalExwTXHRlX-ZBx2xYHso3ffQ0 | docker login -u sergiimarchuk --password-stdin
=======
echo "$DOCKERHUB_TOKEN" | docker login -u sergiimarchuk --password-stdin
>>>>>>> 01c153afcd4da61c184d4d53ed063f29695a52b0
docker pull sergiimarchuk/renovaora-com:latest
#
docker pull sergiimarchuk/renovaora-com:latest
docker stop renovaora_web
docker rm renovaora_web
docker run -d --name renovaora_web -p 127.0.0.1:7878:8080 sergiimarchuk/renovaora-com:latest
