#!/usr/bin/env python3
"""Выгружает названия российских товаров из Open Food Facts для проверки
категоризации на стороннем источнике.

Данные в репозиторий не кладутся: у Open Food Facts лицензия ODbL со
share-alike, а проект обязан работать без них. Скрипт нужен, чтобы проверку
можно было повторить.

    python3 scripts/fetch_openfoodfacts.py > datasets/openfoodfacts_ru.csv
    go run ./cmd/coverage datasets/openfoodfacts_ru.csv
"""

import json
import re
import sys
import time
import urllib.request

# Таксономия Open Food Facts сопоставляется с нашими категориями от
# специфичного к общему: у товара много тегов, первый совпавший считается верным.
MAPPING = [
    ("en:baby-foods", "kids"), ("en:baby-milks", "kids"),
    ("en:pet-food", "pets"),
    ("en:alcoholic-beverages", "alcohol"), ("en:beers", "alcohol"),
    ("en:wines", "alcohol"), ("en:spirits", "alcohol"),
    ("en:coffees", "coffee_tea"), ("en:teas", "coffee_tea"),
    ("en:hot-beverages", "coffee_tea"),
    ("en:ice-creams", "sweets"), ("en:frozen-desserts", "sweets"),
    ("en:chocolates", "sweets"), ("en:candies", "sweets"),
    ("en:confectioneries", "sweets"), ("en:biscuits-and-cakes", "sweets"),
    ("en:sweet-snacks", "sweets"), ("en:crisps", "sweets"),
    ("en:salty-snacks", "sweets"), ("en:nuts", "sweets"),
    ("en:cheeses", "dairy"), ("en:yogurts", "dairy"), ("en:milks", "dairy"),
    ("en:fermented-milk-products", "dairy"), ("en:dairies", "dairy"),
    ("en:eggs", "dairy"),
    ("en:sausages", "meat"), ("en:hams", "meat"), ("en:poultry", "meat"),
    ("en:meats", "meat"), ("en:prepared-meats", "meat"),
    ("en:fishes", "fish"), ("en:seafood", "fish"), ("en:canned-fishes", "fish"),
    ("en:breads", "bakery"), ("en:bread", "bakery"), ("en:viennoiserie", "bakery"),
    ("en:waters", "drinks"), ("en:juices", "drinks"), ("en:sodas", "drinks"),
    ("en:fruit-juices", "drinks"), ("en:beverages", "drinks"),
    ("en:frozen-foods", "frozen"),
    ("en:fresh-vegetables", "vegetables"), ("en:fresh-fruits", "vegetables"),
    ("en:vegetables", "vegetables"), ("en:fruits", "vegetables"),
    ("en:pastas", "grocery"), ("en:rice", "grocery"), ("en:cereals", "grocery"),
    ("en:flours", "grocery"), ("en:sauces", "grocery"), ("en:condiments", "grocery"),
    ("en:vegetable-oils", "grocery"), ("en:fats", "grocery"),
    ("en:canned-foods", "grocery"), ("en:groceries", "grocery"),
]

API = ("https://world.openfoodfacts.org/api/v2/search?countries_tags=russia"
       "&fields=product_name,categories_tags&page_size=100&page=%d")

cyrillic = re.compile(r"[а-яА-ЯёЁ]")


def fetch(page):
    request = urllib.request.Request(API % page, headers={"User-Agent": "chekmate/1.0"})
    with urllib.request.urlopen(request, timeout=60) as response:
        return json.load(response)


def main():
    pages = int(sys.argv[1]) if len(sys.argv) > 1 else 25
    seen = set()

    print("# Названия российских товаров из Open Food Facts (ODbL).")
    print("# Категории сопоставлены с нашими по таксономии OFF.")

    for page in range(1, pages + 1):
        try:
            data = fetch(page)
        except Exception as err:  # сервис отвечает не всегда
            print("# страница %d пропущена: %s" % (page, err), file=sys.stderr)
            continue

        for product in data.get("products", []):
            name = " ".join((product.get("product_name") or "").split())
            tags = product.get("categories_tags") or []

            if not name or len(name) < 6 or ";" in name or not cyrillic.search(name):
                continue
            if name.lower() in seen:
                continue

            category = next((code for tag, code in MAPPING if tag in tags), None)
            if not category:
                continue

            seen.add(name.lower())
            print("%s;%s" % (name, category))

        time.sleep(0.4)


if __name__ == "__main__":
    main()
