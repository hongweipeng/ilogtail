from flask import Flask, request

app = Flask(__name__)


@app.before_request
def before_request():
    print(request.environ, request.data)

@app.route('/')
def hello_world():
    return 'Hello, World!'


if __name__ == '__main__':
    app.run('0.0.0.0', 80)

