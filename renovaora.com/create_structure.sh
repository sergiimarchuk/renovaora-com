#!/bin/bash
set -e

echo "=== Создание только необходимых директорий проекта ==="

# 1. Создаем только папки
mkdir -p cmd/web
mkdir -p internal/handlers
mkdir -p ui/html/pages
mkdir -p ui/html/layouts
mkdir -p ui/static

# 2. Создаем только обязательные для запуска файлы
touch cmd/web/main.go          # Главная точка входа приложения
touch ui/html/layouts/base.html # Базовый шаблон (для меню в центре)
touch ui/html/pages/index.html  # Главная страница

# 3. Создаем пустой go.mod, так как на сервере нет Go
if [ ! -f go.mod ]; then
    touch go.mod
fi

echo "=== Папки и обязательные шаблоны созданы ==="

