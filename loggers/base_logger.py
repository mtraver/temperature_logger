from google.protobuf import timestamp_pb2  # pylint: disable=no-name-in-module

# Auto-generated by protobuf compiler
from loggers import measurement_pb2


class Logger(object):
  """Base class for temperature measurement loggers."""

  def log(self, timestamp, values):
    """Logs the given timestamp and temperature values.

    Args:
      timestamp: A datetime.datetime.
      values: A list of temperature values to log. The subclass can choose to
              log all of them, or if the backend doesn't support that, to
              reduce the list to a single value by some strategy such as
              taking the mean.
    """
    raise NotImplementedError()


class GCPLogger(Logger):
  """Base class for Loggers that log to Google Cloud Platform."""

  def __init__(self, project_id, device_id):
    """Creates a logger for logging to Google Cloud Platform.

    Args:
      project_id: The ID of the Google Cloud project the device belongs to.
      device_id: Device ID string.
    """
    self._project_id = project_id
    self._device_id = device_id

  def _get_proto(self, timestamp, values):
    timestamp_proto = timestamp_pb2.Timestamp()
    timestamp_proto.FromDatetime(timestamp)

    # Just one value can be stored, so take the mean of the given values.
    # TODO(mtraver) Allow user to configure this behavior?
    mean_temp = float(sum(values)) / len(values)

    return measurement_pb2.Measurement(
        device_id=self._device_id, timestamp=timestamp_proto, temp=mean_temp)
