

set -a
source .env
set +a

echo dckr_pat_kalExwTXHRlX-ZBx2xYHso3ffQ0 | docker login -u sergiimarchuk --password-stdin
docker pull sergiimarchuk/renovaora-com:latest
