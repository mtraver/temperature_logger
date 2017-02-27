"""Log temperature in degrees Celsius from MCP9808 sensor.

Temperature can be logged to a file and/or a Google Sheets spreadsheet. If
neither a file nor a spreadsheet is specified, data is logged to stdout.
"""
import argparse
import csv
from datetime import datetime
import os
import time

import Adafruit_MCP9808.MCP9808 as MCP9808

import google_sheets_logger
import util

DEFAULT_NUM_SAMPLES = 1
DEFAULT_SAMPLE_DELAY_SECS = 2

DATE_COL_HEADER = 'Date'


def log_to_csv(filename, data):
  # Write headers if the file doesn't exist or if it's empty
  write_header = not os.path.isfile(filename) or os.stat(filename).st_size == 0

  with open(filename, 'a') as f:
    csv_writer = csv.writer(f)

    if write_header:
      headers = [DATE_COL_HEADER] + ['Temp%d' % (i + 1)
                                     for i in xrange(args.num_samples)]
      csv_writer.writerow(headers)

    csv_writer.writerow(data)


if __name__ == '__main__':
  parser = argparse.ArgumentParser(
      description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)

  logging_group = parser.add_argument_group('Logging')
  logging_group.add_argument('-s', '--sheet_id',
                             type=util.argparse_utils.non_empty_string,
                             help='Google Sheets spreadsheet ID. If given, '
                                  '-k/--keyfile is required. The sheet must '
                                  'be shared with the service account email '
                                  'address associated with the key.')
  logging_group.add_argument('-k', '--keyfile',
                             type=util.argparse_utils.non_empty_string,
                             help='Path to Google API service account JSON key '
                                  'file. If given, -s/--sheet_id is required.')
  logging_group.add_argument('-f', '--log_file',
                             type=util.argparse_utils.non_empty_string,
                             help='CSV file to which to log data')

  sampling_group = parser.add_argument_group('Data sampling')
  sampling_group.add_argument('-n', '--num_samples', type=int,
                              default=DEFAULT_NUM_SAMPLES,
                              help='Number of samples to take. '
                                   'Defaults to %d.' % DEFAULT_NUM_SAMPLES)
  sampling_group.add_argument('-d', '--sample_delay', type=int,
                              default=DEFAULT_SAMPLE_DELAY_SECS,
                              help='Number of seconds to sleep '
                                   'between samples. Defaults '
                                   'to %d.' % DEFAULT_SAMPLE_DELAY_SECS)

  args = parser.parse_args()

  # -k/--keyfile and -s/--sheet_id are mutually inclusive
  if args.keyfile is not None and args.sheet_id is None:
    parser.error('-k/--keyfile requires -s/--sheet_id')
  if args.keyfile is None and args.sheet_id is not None:
    parser.error('-s/--sheet_id requires -k/--keyfile')

  sensor = MCP9808.MCP9808()
  sensor.begin()

  # Construct list of timestamp followed by temperature measurements
  data = [datetime.utcnow().isoformat()]
  for i in xrange(args.num_samples):
    data.append(sensor.readTempC())

    # No need to sleep after last measurement is recorded
    if i < args.num_samples - 1:
      time.sleep(args.sample_delay)

  if args.log_file:
    log_to_csv(args.log_file, data)

  if args.keyfile:
    google_sheets_logger.append_to_sheet(args.keyfile, args.sheet_id, [data])

  # Log to stdout if not logging anywhere else
  if not args.log_file and not args.keyfile:
    print ','.join([str(x) for x in data])
