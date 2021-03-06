package processors

import (
	"testing"

	"github.com/stellar/go/exp/ingest/io"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/toid"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ParticipantsProcessorTestSuiteLedger struct {
	suite.Suite
	processor                        *ParticipantsProcessor
	mockQ                            *history.MockQParticipants
	mockBatchInsertBuilder           *history.MockTransactionParticipantsBatchInsertBuilder
	mockOperationsBatchInsertBuilder *history.MockOperationParticipantBatchInsertBuilder

	firstTx     io.LedgerTransaction
	secondTx    io.LedgerTransaction
	thirdTx     io.LedgerTransaction
	firstTxID   int64
	secondTxID  int64
	thirdTxID   int64
	addresses   []string
	addressToID map[string]int64
	txs         []io.LedgerTransaction
}

func TestParticipantsProcessorTestSuiteLedger(t *testing.T) {
	suite.Run(t, new(ParticipantsProcessorTestSuiteLedger))
}

func (s *ParticipantsProcessorTestSuiteLedger) SetupTest() {
	s.mockQ = &history.MockQParticipants{}
	s.mockBatchInsertBuilder = &history.MockTransactionParticipantsBatchInsertBuilder{}
	s.mockOperationsBatchInsertBuilder = &history.MockOperationParticipantBatchInsertBuilder{}
	sequence := uint32(20)

	s.addresses = []string{
		"GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2",
		"GAXI33UCLQTCKM2NMRBS7XYBR535LLEVAHL5YBN4FTCB4HZHT7ZA5CVK",
		"GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
	}

	s.firstTx = createTransaction(true, 1)
	s.firstTx.Index = 1
	s.firstTx.Envelope.Tx.SourceAccount = xdr.MustAddress(s.addresses[0])
	s.firstTxID = toid.New(int32(sequence), 1, 0).ToInt64()

	s.secondTx = createTransaction(true, 1)
	s.secondTx.Index = 2
	s.secondTx.Envelope.Tx.Operations[0].Body = xdr.OperationBody{
		Type: xdr.OperationTypeCreateAccount,
		CreateAccountOp: &xdr.CreateAccountOp{
			Destination: xdr.MustAddress(s.addresses[1]),
		},
	}
	s.secondTx.Envelope.Tx.SourceAccount = xdr.MustAddress(s.addresses[2])
	s.secondTxID = toid.New(int32(sequence), 2, 0).ToInt64()

	s.thirdTx = createTransaction(true, 1)
	s.thirdTx.Index = 3
	s.thirdTx.Envelope.Tx.SourceAccount = xdr.MustAddress(s.addresses[0])
	s.thirdTxID = toid.New(int32(sequence), 3, 0).ToInt64()

	s.addressToID = map[string]int64{
		s.addresses[0]: 2,
		s.addresses[1]: 20,
		s.addresses[2]: 200,
	}

	s.processor = NewParticipantsProcessor(
		s.mockQ,
		sequence,
	)

	s.txs = []io.LedgerTransaction{
		s.firstTx,
		s.secondTx,
		s.thirdTx,
	}
}

func (s *ParticipantsProcessorTestSuiteLedger) TearDownTest() {
	s.mockQ.AssertExpectations(s.T())
	s.mockBatchInsertBuilder.AssertExpectations(s.T())
	s.mockOperationsBatchInsertBuilder.AssertExpectations(s.T())
}

func (s *ParticipantsProcessorTestSuiteLedger) mockSuccessfulTransactionBatchAdds() {
	s.mockBatchInsertBuilder.On(
		"Add", s.firstTxID, s.addressToID[s.addresses[0]],
	).Return(nil).Once()

	s.mockBatchInsertBuilder.On(
		"Add", s.secondTxID, s.addressToID[s.addresses[1]],
	).Return(nil).Once()
	s.mockBatchInsertBuilder.On(
		"Add", s.secondTxID, s.addressToID[s.addresses[2]],
	).Return(nil).Once()

	s.mockBatchInsertBuilder.On(
		"Add", s.thirdTxID, s.addressToID[s.addresses[0]],
	).Return(nil).Once()
}

func (s *ParticipantsProcessorTestSuiteLedger) mockSuccessfulOperationBatchAdds() {
	s.mockOperationsBatchInsertBuilder.On(
		"Add", s.firstTxID+1, s.addressToID[s.addresses[0]],
	).Return(nil).Once()
	s.mockOperationsBatchInsertBuilder.On(
		"Add", s.secondTxID+1, s.addressToID[s.addresses[1]],
	).Return(nil).Once()
	s.mockOperationsBatchInsertBuilder.On(
		"Add", s.secondTxID+1, s.addressToID[s.addresses[2]],
	).Return(nil).Once()
	s.mockOperationsBatchInsertBuilder.On(
		"Add", s.thirdTxID+1, s.addressToID[s.addresses[0]],
	).Return(nil).Once()
}
func (s *ParticipantsProcessorTestSuiteLedger) TestEmptyParticipants() {
	err := s.processor.Commit()
	s.Assert().NoError(err)
}

func (s *ParticipantsProcessorTestSuiteLedger) TestIngestParticipantsSucceeds() {
	s.mockQ.On("CreateAccounts", mock.AnythingOfType("[]string"), maxBatchSize).
		Run(func(args mock.Arguments) {
			arg := args.Get(0).([]string)
			s.Assert().ElementsMatch(
				s.addresses,
				arg,
			)
		}).Return(s.addressToID, nil).Once()
	s.mockQ.On("NewTransactionParticipantsBatchInsertBuilder", maxBatchSize).
		Return(s.mockBatchInsertBuilder).Once()
	s.mockQ.On("NewOperationParticipantBatchInsertBuilder", maxBatchSize).
		Return(s.mockOperationsBatchInsertBuilder).Once()

	s.mockSuccessfulTransactionBatchAdds()
	s.mockSuccessfulOperationBatchAdds()

	s.mockBatchInsertBuilder.On("Exec").Return(nil).Once()
	s.mockOperationsBatchInsertBuilder.On("Exec").Return(nil).Once()

	for _, tx := range s.txs {
		err := s.processor.ProcessTransaction(tx)
		s.Assert().NoError(err)
	}
	err := s.processor.Commit()
	s.Assert().NoError(err)
}

func (s *ParticipantsProcessorTestSuiteLedger) TestCreateAccountsFails() {
	s.mockQ.On("CreateAccounts", mock.AnythingOfType("[]string"), maxBatchSize).
		Return(s.addressToID, errors.New("transient error")).Once()
	for _, tx := range s.txs {
		err := s.processor.ProcessTransaction(tx)
		s.Assert().NoError(err)
	}
	err := s.processor.Commit()
	s.Assert().EqualError(err, "Could not create account ids: transient error")
}

func (s *ParticipantsProcessorTestSuiteLedger) TestBatchAddFails() {
	s.mockQ.On("CreateAccounts", mock.AnythingOfType("[]string"), maxBatchSize).
		Run(func(args mock.Arguments) {
			arg := args.Get(0).([]string)
			s.Assert().ElementsMatch(
				s.addresses,
				arg,
			)
		}).Return(s.addressToID, nil).Once()
	s.mockQ.On("NewTransactionParticipantsBatchInsertBuilder", maxBatchSize).
		Return(s.mockBatchInsertBuilder).Once()

	s.mockBatchInsertBuilder.On(
		"Add", s.firstTxID, s.addressToID[s.addresses[0]],
	).Return(errors.New("transient error")).Once()

	s.mockBatchInsertBuilder.On(
		"Add", s.secondTxID, s.addressToID[s.addresses[1]],
	).Return(nil).Maybe()
	s.mockBatchInsertBuilder.On(
		"Add", s.secondTxID, s.addressToID[s.addresses[2]],
	).Return(nil).Maybe()

	s.mockBatchInsertBuilder.On(
		"Add", s.thirdTxID, s.addressToID[s.addresses[0]],
	).Return(nil).Maybe()
	for _, tx := range s.txs {
		err := s.processor.ProcessTransaction(tx)
		s.Assert().NoError(err)
	}
	err := s.processor.Commit()
	s.Assert().EqualError(err, "Could not insert transaction participant in db: transient error")
}

func (s *ParticipantsProcessorTestSuiteLedger) TestOperationParticipantsBatchAddFails() {
	s.mockQ.On("CreateAccounts", mock.AnythingOfType("[]string"), maxBatchSize).
		Run(func(args mock.Arguments) {
			arg := args.Get(0).([]string)
			s.Assert().ElementsMatch(
				s.addresses,
				arg,
			)
		}).Return(s.addressToID, nil).Once()
	s.mockQ.On("NewTransactionParticipantsBatchInsertBuilder", maxBatchSize).
		Return(s.mockBatchInsertBuilder).Once()
	s.mockQ.On("NewOperationParticipantBatchInsertBuilder", maxBatchSize).
		Return(s.mockOperationsBatchInsertBuilder).Once()

	s.mockSuccessfulTransactionBatchAdds()

	s.mockOperationsBatchInsertBuilder.On(
		"Add", s.firstTxID+1, s.addressToID[s.addresses[0]],
	).Return(errors.New("transient error")).Once()
	s.mockOperationsBatchInsertBuilder.On(
		"Add", s.secondTxID+1, s.addressToID[s.addresses[1]],
	).Return(nil).Maybe()
	s.mockOperationsBatchInsertBuilder.On(
		"Add", s.secondTxID+1, s.addressToID[s.addresses[2]],
	).Return(nil).Maybe()
	s.mockOperationsBatchInsertBuilder.On(
		"Add", s.thirdTxID+1, s.addressToID[s.addresses[0]],
	).Return(nil).Maybe()

	s.mockBatchInsertBuilder.On("Exec").Return(nil).Once()

	for _, tx := range s.txs {
		err := s.processor.ProcessTransaction(tx)
		s.Assert().NoError(err)
	}
	err := s.processor.Commit()
	s.Assert().EqualError(err, "could not insert operation participant in db: transient error")
}

func (s *ParticipantsProcessorTestSuiteLedger) TestBatchAddExecFails() {
	s.mockQ.On("CreateAccounts", mock.AnythingOfType("[]string"), maxBatchSize).
		Run(func(args mock.Arguments) {
			arg := args.Get(0).([]string)
			s.Assert().ElementsMatch(
				s.addresses,
				arg,
			)
		}).Return(s.addressToID, nil).Once()
	s.mockQ.On("NewTransactionParticipantsBatchInsertBuilder", maxBatchSize).
		Return(s.mockBatchInsertBuilder).Once()

	s.mockSuccessfulTransactionBatchAdds()

	s.mockBatchInsertBuilder.On("Exec").Return(errors.New("transient error")).Once()

	for _, tx := range s.txs {
		err := s.processor.ProcessTransaction(tx)
		s.Assert().NoError(err)
	}
	err := s.processor.Commit()
	s.Assert().EqualError(err, "Could not flush transaction participants to db: transient error")
}

func (s *ParticipantsProcessorTestSuiteLedger) TestOpeartionBatchAddExecFails() {
	s.mockQ.On("CreateAccounts", mock.AnythingOfType("[]string"), maxBatchSize).
		Run(func(args mock.Arguments) {
			arg := args.Get(0).([]string)
			s.Assert().ElementsMatch(
				s.addresses,
				arg,
			)
		}).Return(s.addressToID, nil).Once()
	s.mockQ.On("NewTransactionParticipantsBatchInsertBuilder", maxBatchSize).
		Return(s.mockBatchInsertBuilder).Once()
	s.mockQ.On("NewOperationParticipantBatchInsertBuilder", maxBatchSize).
		Return(s.mockOperationsBatchInsertBuilder).Once()

	s.mockSuccessfulTransactionBatchAdds()
	s.mockSuccessfulOperationBatchAdds()

	s.mockBatchInsertBuilder.On("Exec").Return(nil).Once()
	s.mockOperationsBatchInsertBuilder.On("Exec").Return(errors.New("transient error")).Once()

	for _, tx := range s.txs {
		err := s.processor.ProcessTransaction(tx)
		s.Assert().NoError(err)
	}
	err := s.processor.Commit()
	s.Assert().EqualError(err, "could not flush operation participants to db: transient error")
}
