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

import React, { useState } from 'react';
import DreamStudioPanel from './DreamStudioPanel';

const DreamStudio = () => {
  const [prompt, setPrompt] = useState('');
  const [model, setModel] = useState('');
  const [resolution, setResolution] = useState('1024x1024');
  const [aspectRatio, setAspectRatio] = useState('1:1');
  const [referenceImage, setReferenceImage] = useState(null);
  const [count, setCount] = useState(1);

  return (
    <div style={{ marginTop: '60px' }}>
      <DreamStudioPanel
        prompt={prompt}
        setPrompt={setPrompt}
        model={model}
        setModel={setModel}
        resolution={resolution}
        setResolution={setResolution}
        aspectRatio={aspectRatio}
        setAspectRatio={setAspectRatio}
        referenceImage={referenceImage}
        setReferenceImage={setReferenceImage}
        count={count}
        setCount={setCount}
      />
    </div>
  );
};

export default DreamStudio;
