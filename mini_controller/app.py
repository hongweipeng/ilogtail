import os
import sys
from flask import Flask, request

sys.path.append(os.path.join(os.path.dirname(__file__), 'protos'))

from protos.opentelemetry.proto.collector.logs.v1 import logs_service_pb2

app = Flask(__name__)


@app.before_request
def before_request():
    print(request.environ, request.data)

@app.route('/')
def hello_world():
    return 'Hello, World!'

@app.route('/otupload', methods=['POST'])
def hello_otupload():
    data = request.data
    msg = logs_service_pb2.ExportLogsServiceRequest()
    msg.ParseFromString(data)
    print(str(msg))
    return 'ok'


if __name__ == '__main__':
    app.run('0.0.0.0', 80)

