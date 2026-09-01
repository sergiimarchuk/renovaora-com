echo dckr_pat_kalExwTXHRlX-ZBx2xYHso3ffQ0 | docker login -u sergiimarchuk --password-stdin
docker build -t sergiimarchuk/renovaora-com:latest .
docker push sergiimarchuk/renovaora-com:latest
