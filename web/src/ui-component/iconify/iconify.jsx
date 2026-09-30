import PropTypes from 'prop-types';
import { forwardRef } from 'react';
import { Icon, disableCache } from '@iconify/react';

import Box from '@mui/material/Box';

// ----------------------------------------------------------------------

const Iconify = forwardRef(({ width = 20, sx, ...other }, ref) => (
  <Box
    ssr
    ref={ref}
    component={Icon}
    sx={{
      width,
      height: width,
      flexShrink: 0,
      display: 'inline-flex',
      ...sx
    }}
    {...other}
  />
));

disableCache('local');

Iconify.propTypes = {
  width: PropTypes.oneOfType([PropTypes.number, PropTypes.string]),
  sx: PropTypes.object
};

export default Iconify;
