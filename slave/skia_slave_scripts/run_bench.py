#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Run the Skia benchmarking executable."""

import os
import sys

from build_step import BuildStep
from utils import gclient_utils


def BenchArgs(data_file):
  """Builds a list containing arguments to pass to bench.

  Args:
    data_file: filepath to store the log output
  Returns:
    list containing arguments to pass to bench
  """
  return ['--timers', 'wg', '--logFile', data_file]

# Device name -> extra arguments. Name can be full builder name or a fragment.
EXTRA_ARGS = {
    'GalaxyNexus': ['--match', '~DeferredSurfaceCopy'],  # Crash: skbug.com/1687
    'Nexus4': ['--config', 'defaults', 'MSAA4'],
    'NexusS': ['--match', '~DeferredSurfaceCopy'],       # Crash: skbug.com/1687
    'Valgrind': ['--runOnce', 'true', '--config', '8888', 'GPU',
                 'NONRENDERING'],
    'Perf-Win8-ShuttleA-GTX660-x86-Release': [
        '--match', '~giantdashline'],                    # Crash: skbug.com/2505
}


class RunBench(BuildStep):
  """A BuildStep that runs bench."""

  def __init__(self, timeout=9600, no_output_timeout=9600, **kwargs):
    super(RunBench, self).__init__(timeout=timeout,
                                   no_output_timeout=no_output_timeout,
                                   **kwargs)

  def _BuildDataFile(self):
    return os.path.join(self._device_dirs.PerfDir(),
                        'bench_%s_data' % self._got_revision)

  def _BuildJSONDataFile(self):
    git_timestamp = gclient_utils.GetGitRepoPOSIXTimestamp()
    return os.path.join(
        self._device_dirs.PerfDir(), 'microbench_%s_%s.json' % (
            self._got_revision, git_timestamp))

  def _Run(self):
    args = ['-i', self._device_dirs.ResourceDir()]
    if self._perf_data_dir:
      args.extend(BenchArgs(self._BuildDataFile()))
      args.extend(['--outResultsFile', self._BuildJSONDataFile()])
      # Add --outResultsFile flag here because it breaks bench_pictures.py
      # if it was placed in there.
    for builder_name_or_fragment in EXTRA_ARGS:
      if builder_name_or_fragment in self._builder_name:
        args.extend(EXTRA_ARGS[builder_name_or_fragment])
    self._flavor_utils.RunFlavoredCmd('bench', args + self._bench_args)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunBench))
