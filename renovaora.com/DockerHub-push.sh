echo dckr_pat_3obGjTBpMlUnjjI1rc2pZi18Xeg | docker login -u sergiimarchuk --password-stdin
docker build -t sergiimarchuk/renovaora-com:latest .
docker push sergiimarchuk/renovaora-com:latest
