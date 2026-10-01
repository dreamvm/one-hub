import React, { useEffect, useState } from 'react';
import { showError } from 'utils/common';
import { API } from 'utils/api';
import BaseIndex from './baseIndex';
import { Box } from '@mui/material';
import { useTranslation } from 'react-i18next';
import ContentViewer from 'ui-component/ContentViewer';

const Home = () => {
  const { t } = useTranslation();
  const [homePageContentLoaded, setHomePageContentLoaded] = useState(false);
  const [homePageContent, setHomePageContent] = useState('');

  const [homePageContentError, setHomePageContentError] = useState(false);

  useEffect(() => {
    let active = true;
    const displayHomePageContent = async () => {
      try {
        const res = await API.get('/api/home_page_content');
        if (!active) return;
        const { success, message, data } = res.data;
        if (success) {
          setHomePageContent(data);
          localStorage.setItem('home_page_content', data);
        } else {
          showError(message);
          setHomePageContentError(true);
        }
      } catch (error) {
        if (active) setHomePageContentError(true);
      } finally {
        if (active) setHomePageContentLoaded(true);
      }
    };

    displayHomePageContent();
    return () => {
      active = false;
    };
  }, []);

  return (
    <>
      {homePageContentLoaded && !homePageContentError && homePageContent === '' ? (
        <BaseIndex />
      ) : (
        <Box>
          <ContentViewer
            content={homePageContent}
            loading={!homePageContentLoaded}
            errorMessage={homePageContentError ? t('home.loadingErr') : ''}
            containerStyle={{ minHeight: 'calc(100vh - 136px)' }}
            contentStyle={{ fontSize: 'larger' }}
          />
        </Box>
      )}
    </>
  );
};

export default Home;
