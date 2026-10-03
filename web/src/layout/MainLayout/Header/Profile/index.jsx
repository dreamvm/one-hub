import PropTypes from 'prop-types';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
// material-ui
import { useTheme } from '@mui/material/styles';
import { Avatar, IconButton } from '@mui/material';
import User1 from 'assets/images/users/user-round.svg';

// ==============================|| PROFILE MENU ||============================== //

const Profile = ({ toggleProfileDrawer }) => {
  const theme = useTheme();
  const { t } = useTranslation();
  const account = useSelector((state) => state.account);

  return (
    <>
      {/* 用户头像按钮 */}
      <IconButton
        aria-label={t('profile')}
        onClick={toggleProfileDrawer}
        sx={{
          cursor: 'pointer',
          position: 'relative',
          width: '48px',
          height: '48px',
          padding: 0,
          display: 'flex',
          justifyContent: 'center',
          alignItems: 'center',
          borderRadius: '50%',
          background: `linear-gradient(90deg, 
            ${theme.palette.primary.main}, 
            ${theme.palette.secondary.main}, 
            ${theme.palette.primary.light}, 
            ${theme.palette.primary.main})`
        }}
      >
        <Avatar
          src={account.user?.avatar_url || User1}
          sx={{
            ...theme.typography.mediumAvatar,
            cursor: 'pointer',
            width: '45px',
            height: '45px',
            border: '1px solid',
            borderColor: (theme) => (theme.palette.mode === 'dark' ? theme.palette.background.paper : '#ffffff'),
            bgcolor: '#FFFFFF',
            variant: 'rounded',
            transition: 'transform 0.2s ease-in-out, background-color 0.2s ease-in-out',
            '&:hover': {
              transform: 'scale(1.03)'
            }
          }}
        />
      </IconButton>
    </>
  );
};

Profile.propTypes = {
  toggleProfileDrawer: PropTypes.func
};

export default Profile;
