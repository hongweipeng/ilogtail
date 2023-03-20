from flask import Flask, request
import sls_logs_pb2
app = Flask(__name__)


@app.before_request
def before_request():
    print(request.environ, request.data)

@app.route('/')
def hello_world():
    return 'Hello, World!'

@app.route('/logstores/test_logstore/shards/lb', methods=['POST'])
def test_logstore():
    # body = request.data
    # msg = sls_logs_pb2.LogGroup()
    # msg.ParseFromString(body)
    # print(str(msg))
    return 'ok'


if __name__ == '__main__':
    app.run('0.0.0.0', 80)

