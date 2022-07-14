import { Center } from "@chakra-ui/react";
import React, { useEffect, useState, useRef } from "react";
import useToken from "../../hooks/useToken";
import useApi from "../../hooks/useApi";

const AdminSettings: React.FC = () => {
  return (
    <>
      <Center h="calc(100%-4rem)" w={"85vw"}>
        AdminSettings
      </Center>
    </>
  );
};

export default AdminSettings;
