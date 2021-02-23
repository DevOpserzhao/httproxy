import requests


def get(server):
    res = requests.get(server, headers={"Host": "www.yebaojiasu.com"})
    print(res.text)


if __name__ == '__main__':
    get("http://219.135.99.130")
    # get("http://127.0.0.1")

