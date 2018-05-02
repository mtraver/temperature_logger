import re

from google.protobuf import timestamp_pb2  # pylint: disable=no-name-in-module

# Auto-generated by protobuf compiler
from . import measurement_pb2


class InvalidProtoError(Exception):
  pass


def validate_measurement(measurement):
  """Validates a Measurement protobuf.

  If the proto is invalid, InvalidProtoError is raised. If it's valid then
  no exception is raised and nothing is returned.

  Validation is performed based on custom protobuf field options. At the
  moment this is just the "regex" option.

  Args:
    measurement: A measurement_pb2.Measurement.

  Raises:
    InvalidProtoError: If the given proto is invalid.
  """
  for field_desc, value in measurement.ListFields():
    options = field_desc.GetOptions()

    # Validate field based on regex option
    if options.HasExtension(measurement_pb2.regex):
      regex = options.Extensions[measurement_pb2.regex]
      if not re.match(regex, value):
        raise InvalidProtoError(
            'Field failed regex validation. Field: "{}" Value: "{}" '
            'Regex: "{}"'.format(field_desc.name, value, regex))


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
    """Returns a Measurement protobuf made from the given timestamp and values.

    Args:
      timestamp: A datetime.datetime.
      values: A list of temperature values to log. If there is more than one
              value in the list the mean is taken and that value is put in the
              protobuf.

    Returns:
      A measurement_pb2.Measurement.

    Raises:
      InvalidProtoError: If the Measurement protobuf doesn't pass validation.
    """
    timestamp_proto = timestamp_pb2.Timestamp()
    timestamp_proto.FromDatetime(timestamp)

    # Just one value can be stored, so take the mean of the given values.
    # TODO(mtraver) Allow user to configure this behavior?
    mean_temp = sum(values) / len(values)

    proto = measurement_pb2.Measurement(
        device_id=self._device_id, timestamp=timestamp_proto, temp=mean_temp)

    # This will raise if the proto is invalid. Let the caller handle it.
    validate_measurement(proto)

    return proto