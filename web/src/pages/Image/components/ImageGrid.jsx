/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useRef, useEffect, useState, memo } from 'react';
import { Skeleton, Tooltip, Typography } from '@douyinfe/semi-ui';
import { useIsMobile } from '../../../hooks/common/useIsMobile';

const { Text } = Typography;

// 单个图片项组件
const ImageItem = memo(({ image, index, onClick }) => {
  const [isLoaded, setIsLoaded] = useState(false);
  const [isVisible, setIsVisible] = useState(false);
  const imgRef = useRef(null);

  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            setIsVisible(true);
            observer.unobserve(entry.target);
          }
        });
      },
      {
        rootMargin: '50px', // 提前50px开始加载
        threshold: 0.01,
      }
    );

    if (imgRef.current) {
      observer.observe(imgRef.current);
    }

    return () => {
      if (imgRef.current) {
        observer.unobserve(imgRef.current);
      }
    };
  }, []);

  const handleImageLoad = () => {
    setIsLoaded(true);
  };

  const handleClick = () => {
    onClick(image, index);
  };

  return (
    <div
      ref={imgRef}
      className="image-grid-item"
      style={{
        position: 'relative',
        paddingBottom: '100%', // 1:1 aspect ratio
        backgroundColor: '#f5f5f5',
        borderRadius: '8px',
        overflow: 'hidden',
        cursor: 'pointer',
        transition: 'transform 0.2s, box-shadow 0.2s',
      }}
      onClick={handleClick}
      onMouseEnter={(e) => {
        e.currentTarget.style.transform = 'translateY(-4px)';
        e.currentTarget.style.boxShadow = '0 4px 12px rgba(0, 0, 0, 0.15)';
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.transform = 'translateY(0)';
        e.currentTarget.style.boxShadow = 'none';
      }}
    >
      {!isLoaded && (
        <div
          style={{
            position: 'absolute',
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
          }}
        >
          <Skeleton.Image
            style={{
              width: '100%',
              height: '100%',
            }}
          />
        </div>
      )}

      {isVisible && (
        <img
          src={image.result_url}
          alt={image.prompt}
          style={{
            position: 'absolute',
            top: 0,
            left: 0,
            width: '100%',
            height: '100%',
            objectFit: 'cover',
            opacity: isLoaded ? 1 : 0,
            transition: 'opacity 0.3s',
          }}
          onLoad={handleImageLoad}
          loading="lazy"
        />
      )}

      {/* 悬浮信息层 */}
      {isLoaded && (
        <div
          className="image-overlay"
          style={{
            position: 'absolute',
            bottom: 0,
            left: 0,
            right: 0,
            background: 'linear-gradient(to top, rgba(0,0,0,0.7), transparent)',
            padding: '12px',
            opacity: 0,
            transition: 'opacity 0.2s',
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.opacity = 1;
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.opacity = 0;
          }}
        >
          <Tooltip content={image.prompt} position="top">
            <Text
              ellipsis
              style={{
                color: 'white',
                fontSize: '12px',
                display: 'block',
                marginBottom: '4px',
              }}
            >
              {image.prompt}
            </Text>
          </Tooltip>
          <Text
            style={{
              color: 'rgba(255,255,255,0.8)',
              fontSize: '11px',
              display: 'block',
            }}
          >
            {image.model}
          </Text>
        </div>
      )}
    </div>
  );
});

ImageItem.displayName = 'ImageItem';

// 图片网格组件
const ImageGrid = ({ images, onImageClick }) => {
  const isMobile = useIsMobile();

  // 响应式列数：桌面4列，平板3列，手机2列
  const getGridColumns = () => {
    if (isMobile) return 2;
    if (window.innerWidth < 1024) return 3;
    return 4;
  };

  const [columns, setColumns] = useState(getGridColumns());

  useEffect(() => {
    const handleResize = () => {
      setColumns(getGridColumns());
    };

    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, [isMobile]);

  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: `repeat(${columns}, 1fr)`,
        gap: isMobile ? '12px' : '16px',
        width: '100%',
      }}
    >
      {images.map((image, index) => (
        <ImageItem
          key={image.id}
          image={image}
          index={index}
          onClick={onImageClick}
        />
      ))}
    </div>
  );
};

export default memo(ImageGrid);
